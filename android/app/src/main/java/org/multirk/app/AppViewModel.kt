package org.multirk.app

import android.app.Application
import android.content.Context
import android.net.wifi.WifiManager
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.multirk.app.data.Artifact
import org.multirk.app.data.AgentInstaller
import org.multirk.app.data.AuthRequiredException
import org.multirk.app.data.CtlState
import org.multirk.app.data.Device
import org.multirk.app.data.DeviceInfo
import org.multirk.app.data.Discovery
import org.multirk.app.data.Game
import org.multirk.app.data.MultirkClient
import org.multirk.app.data.Store
import org.multirk.app.data.isConnectionLoss
import org.multirk.app.data.SyncEvent
import org.multirk.app.data.SystemInfo
import org.multirk.app.data.parseDeviceInfo
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.net.SocketTimeoutException

enum class Screen { History, Start, Catalog, Game, Controls, Settings }

const val PAGE_SIZE = 6

data class AgentState(
    val visible: Boolean = false,
    val running: Boolean = false,
    val log: List<String> = emptyList(),
    val host: String = "",
    val installed: Boolean = false,
    val uninstalled: Boolean = false,
    val error: String? = null,
    val mode: String = "add",
    /** Console being unpaired when mode == "unpair". */
    val deviceId: String? = null
)

data class ScraperCreds(
    val user: String = "",
    val pass: String = "",
    val devId: String = "",
    val devPass: String = ""
)

data class AuditSystem(val system: String, val removed: Int, val entries: List<String>)

data class AuditReport(val checked: Int, val removed: Int, val systems: List<AuditSystem>)

data class GameMeta(
    val name: String = "",
    val desc: String = "",
    val developer: String = "",
    val publisher: String = "",
    val genre: String = "",
    val players: String = "",
    val year: String = "",
    val rating: String = ""
)

data class UiState(
    val screen: Screen = Screen.Start,
    val consoleMode: Int = 0,
    val devices: List<Device> = emptyList(),
    val selected: Device? = null,
    val originId: String? = null,
    val targetId: String? = null,
    val systems: List<SystemInfo> = emptyList(),
    val games: List<Game> = emptyList(),
    val filterSystem: String? = null,
    val query: String = "",
    val onlyState: Boolean = false,
    val page: Int = 0,
    val total: Int = 0,
    val game: Game? = null,
    val artifacts: List<Artifact> = emptyList(),
    val scanning: Boolean = false,
    val busy: Boolean = false,
    val busyText: String = "",
    val message: String? = null,
    val pairDevice: Device? = null,
    val manualDialog: Boolean = false,
    val ctl: CtlState = CtlState(),
    val ctlDevice: Device? = null,
    val log: List<SyncEvent> = emptyList(),
    val agent: AgentState = AgentState(),
    val infoDevice: Device? = null,
    val infoData: DeviceInfo? = null,
    val infoLoading: Boolean = false,
    val infoError: String? = null,
    /** Ids that answered the last discovery scan; null until the first scan ends. */
    val online: Set<String>? = null,
    /** Consoles picked on the history screen, in tap order (first = origin). */
    val selection: List<String> = emptyList(),
    val scraper: ScraperCreds = ScraperCreds(),
    val scraping: Boolean = false,
    val auditReport: AuditReport? = null,
    val hiddenSystems: List<SystemInfo> = emptyList(),
    val playing: String? = null,
    /** Console that stopped answering: a blocking dialog offers to reconnect. */
    val lostDevice: Device? = null,
    val reconnecting: Boolean = false,
    val reconnectFailed: Boolean = false,
    val suggestions: List<Game> = emptyList(),
    val scrapingText: String = "Scrapeando… puede tardar unos segundos",
    /** Bumped when a cover changes on disk so the UI reloads it. */
    val coverVersion: Int = 0
) {
    val paired: List<Device> get() = devices.filter { it.paired }
}

class AppViewModel(app: Application) : AndroidViewModel(app) {

    private val store = Store(app)
    private val _state = MutableStateFlow(UiState())
    val state: StateFlow<UiState> = _state.asStateFlow()

    private var lastScanAt = 0L

    private val scraperPrefs =
        app.getSharedPreferences("scraper", Context.MODE_PRIVATE)

    fun setScraper(c: ScraperCreds) {
        scraperPrefs.edit()
            .putString("user", c.user.trim()).putString("pass", c.pass)
            .putString("devid", c.devId.trim()).putString("devpass", c.devPass)
            .apply()
        _state.update { it.copy(scraper = c.copy(user = c.user.trim(), devId = c.devId.trim())) }
        toast("CUENTA GUARDADA")
    }

    init {
        _state.update {
            it.copy(
                scraper = ScraperCreds(
                    scraperPrefs.getString("user", "").orEmpty(),
                    scraperPrefs.getString("pass", "").orEmpty(),
                    scraperPrefs.getString("devid", "").orEmpty(),
                    scraperPrefs.getString("devpass", "").orEmpty()
                )
            )
        }
        viewModelScope.launch(Dispatchers.IO) {
            val stored = store.devices()
            val log = store.log()
            val pair = stored.filter { it.paired }
            val origin = pair.firstOrNull()
            val target = pair.firstOrNull { it.id != origin?.id }
            _state.update {
                it.copy(
                    devices = stored,
                    log = log,
                    selected = origin,
                    originId = origin?.id,
                    targetId = target?.id
                )
            }
            refreshDeviceNames()
        }
    }

    /** The mode is asked on every launch and never persisted. */
    fun chooseConsoleCount(count: Int) {
        _state.update { it.copy(consoleMode = count, screen = Screen.History) }
    }

    fun setConsoleMode(count: Int) {
        _state.update { it.copy(consoleMode = count) }
    }

    private var selectionJob: kotlinx.coroutines.Job? = null

    /**
     * History-screen tap on a console panel.
     * Mode 1: it is the console, continue at once.
     * Mode 2: first tap = origin, second = destination, then continue after a
     * short pause. Tapping a selected console again undoes it.
     */
    fun selectConsole(device: Device) {
        if (!device.paired) {
            showPairDialog(device)
            return
        }
        val s = _state.value
        if (s.consoleMode == 1) {
            selectionJob?.cancel()
            _state.update { it.copy(selection = emptyList(), targetId = null) }
            openDevice(device)
            return
        }

        val next = if (device.id in s.selection) {
            s.selection - device.id
        } else if (s.selection.size < 2) {
            s.selection + device.id
        } else {
            s.selection
        }
        _state.update { it.copy(selection = next) }

        selectionJob?.cancel()
        if (next.size == 2) {
            selectionJob = viewModelScope.launch {
                kotlinx.coroutines.delay(1000)
                val cur = _state.value
                if (cur.selection == next && cur.screen == Screen.History) {
                    val origin = cur.devices.firstOrNull { it.id == next[0] }
                    _state.update {
                        it.copy(
                            originId = next[0],
                            targetId = next[1],
                            selection = emptyList()
                        )
                    }
                    if (origin != null) openDevice(origin)
                }
            }
        }
    }

    // (sin límite de consolas emparejadas)

    fun navigate(screen: Screen) {
        if (screen == Screen.History || screen == Screen.Start) {
            selectionJob?.cancel()
            _state.update { it.copy(screen = screen, selection = emptyList()) }
        } else {
            _state.update { it.copy(screen = screen) }
        }
    }

    fun clearMessage() {
        _state.update { it.copy(message = null) }
    }

    fun showManualDialog(show: Boolean) {
        _state.update { it.copy(manualDialog = show) }
    }

    // ------------------------------------------------------------- agent ----

    fun showAgentDialog(show: Boolean) {
        _state.update { it.copy(agent = AgentState(visible = show)) }
    }

    fun showAddConsole() {
        _state.update { it.copy(agent = AgentState(visible = true, mode = "add")) }
    }

    fun showUninstallDialog(device: Device) {
        _state.update {
            it.copy(
                agent = AgentState(
                    visible = true,
                    mode = "uninstall",
                    host = device.host
                )
            )
        }
    }

    /** Unpairing removes the agent first, then forgets the console. */
    fun startUnpair(device: Device) {
        _state.update {
            it.copy(
                infoDevice = null,
                infoData = null,
                infoError = null,
                infoLoading = false,
                agent = AgentState(
                    visible = true,
                    mode = "unpair",
                    host = device.host,
                    deviceId = device.id
                )
            )
        }
    }

    /** Fallback when the console cannot be reached over SSH. */
    fun unpairWithoutAgent() {
        val id = _state.value.agent.deviceId ?: return
        val device = _state.value.devices.firstOrNull { it.id == id }
        _state.update { it.copy(agent = AgentState()) }
        if (device != null) unpair(device)
    }

    fun closeAgentDialog() {
        _state.update { it.copy(agent = AgentState()) }
    }

    fun installAgent(host: String, user: String, password: String) {
        val h = host.trim()
        if (h.isEmpty()) {
            toast("FALTA LA IP DE LA CONSOLA")
            return
        }
        _state.update {
            it.copy(
                agent = AgentState(
                    visible = true,
                    running = true,
                    host = h,
                    log = listOf("iniciando instalación…")
                )
            )
        }
        viewModelScope.launch(Dispatchers.IO) {
            try {
                val installer = AgentInstaller(h, user.ifBlank { "ark" }, password.ifBlank { "ark" })
                val result = installer.install(
                    getApplication<Application>().assets,
                    h
                ) { line -> appendAgentLog(line) }

                val all = store.devices()
                val already = all.firstOrNull {
                    it.paired && it.host.equals(h, ignoreCase = true) && it.port == 47820
                }
                if (already != null) {
                    _state.update { it.copy(agent = AgentState()) }
                    toast("AGENTE ACTUALIZADO · YA EMPAREJADA")
                    return@launch
                }

                _state.update { it.copy(agent = AgentState()) }
                quickConnect(h, 47820)
            } catch (e: Exception) {
                _state.update {
                    it.copy(agent = it.agent.copy(running = false, error = e.message ?: "ERROR SSH"))
                }
            }
        }
    }

    fun uninstallAgent(host: String, user: String, password: String, purge: Boolean) {
        val h = host.trim()
        if (h.isEmpty()) {
            toast("FALTA LA IP DE LA CONSOLA")
            return
        }
        val before = _state.value.agent
        val unpairing = before.mode == "unpair"
        val deviceId = before.deviceId
        _state.update {
            it.copy(
                agent = AgentState(
                    visible = true,
                    running = true,
                    host = h,
                    mode = if (unpairing) "unpair" else "uninstall",
                    deviceId = deviceId,
                    log = listOf("iniciando desinstalación…")
                )
            )
        }
        viewModelScope.launch(Dispatchers.IO) {
            try {
                AgentInstaller(h, user.ifBlank { "ark" }, password.ifBlank { "ark" })
                    .uninstall(purge || unpairing) { line -> appendAgentLog(line) }
                if (unpairing) {
                    val device = store.devices().firstOrNull { it.id == deviceId }
                    _state.update { it.copy(agent = AgentState()) }
                    if (device != null) unpair(device)
                } else {
                    _state.update {
                        it.copy(agent = it.agent.copy(running = false, uninstalled = true))
                    }
                }
            } catch (e: Exception) {
                _state.update {
                    it.copy(agent = it.agent.copy(running = false, error = e.message ?: "ERROR SSH"))
                }
            }
        }
    }

    private fun appendAgentLog(line: String) {
        _state.update {
            it.copy(agent = it.agent.copy(log = (it.agent.log + line).takeLast(40)))
        }
    }

    fun showPairDialog(device: Device?) {
        if (device == null) return
        _state.update { it.copy(busy = true, busyText = "ABRIENDO ${device.name.uppercase()}…") }
        viewModelScope.launch(Dispatchers.IO) {
            try {
                _state.update { it.copy(busy = false) }
                quickConnect(device.host, device.port)
                kotlinx.coroutines.delay(1500)
                store.devices().firstOrNull { it.host == device.host && it.paired }
                    ?.let { selectConsole(it) }
            } catch (e: Exception) {
                _state.update { it.copy(busy = false) }
                toast("NO SE PUDO EMPAREJAR")
            }
        }
    }

    fun setFilter(system: String?, query: String, onlyState: Boolean, suggest: Boolean = true) {
        _state.update {
            it.copy(
                filterSystem = system, query = query, onlyState = onlyState, page = 0,
                suggestions = if (suggest) it.suggestions else emptyList()
            )
        }
        loadPage()
        val device = _state.value.selected
        if (suggest && device != null) {
            viewModelScope.launch(Dispatchers.IO) {
                val list = store.suggest(device.id, system, query)
                _state.update { if (it.query == query) it.copy(suggestions = list) else it }
            }
        }
    }

    /** Autocomplete tap: fill the box with that name and hide the list. */
    fun pickSuggestion(game: Game) {
        val s = _state.value
        setFilter(s.filterSystem, game.name, s.onlyState, suggest = false)
    }

    fun setPage(page: Int) {
        _state.update { it.copy(page = maxOf(0, page)) }
        loadPage()
    }

    // ---------------------------------------------------------- discovery ---

    fun scan(force: Boolean = false) {
        if (_state.value.scanning) return
        val now = System.currentTimeMillis()
        if (!force && now - lastScanAt < 8_000) return
        lastScanAt = now
        _state.update {
            // A manual refresh shows every console as "searching" again; each
            // one turns active the moment it answers.
            it.copy(scanning = true, online = if (force) emptySet() else it.online)
        }
        viewModelScope.launch(Dispatchers.IO) {
            val lock = acquireMulticastLock()
            val t0 = System.currentTimeMillis()
            val found = try {
                Discovery.scan(store.devices()) { d ->
                    android.util.Log.i(
                        "multirk-scan",
                        "  ${d.host} answered after ${System.currentTimeMillis() - t0} ms"
                    )
                    _state.update { st -> st.copy(online = (st.online ?: emptySet()) + d.id) }
                }
            } catch (e: Exception) {
                emptyList<Device>()
            } finally {
                runCatching { lock?.release() }
            }
            android.util.Log.i(
                "multirk-scan",
                "scan ${System.currentTimeMillis() - t0} ms, found ${found.size}: " +
                    found.joinToString { it.host }
            )

            val merged = LinkedHashMap<String, Device>()
            for (d in store.devices()) merged[d.id] = d
            for (f in found) {
                val sameAddr = merged.values.firstOrNull {
                    it.id != f.id && it.host == f.host && it.port == f.port
                }
                val old = merged[f.id]
                val base = old ?: sameAddr ?: f
                // Prefer the hardware model over generic names like "RG351V".
                val hwName = f.model.takeIf {
                    it.isNotBlank() && !it.equals("rk3326", ignoreCase = true)
                }
                val name = when {
                    hwName != null && isAutoName(base.name) -> hwName
                    f.name.isNotBlank() && isAutoName(base.name) -> f.name
                    else -> base.name
                }
                merged[f.id] = base.copy(
                    id = f.id,
                    name = name,
                    host = f.host.ifBlank { base.host },
                    port = f.port,
                    model = f.model.ifBlank { base.model },
                    systems = maxOf(base.systems, f.systems),
                    games = maxOf(base.games, f.games),
                    lastSeen = System.currentTimeMillis()
                )
                if (sameAddr != null && sameAddr.id != f.id) {
                    merged.remove(sameAddr.id)
                    store.deleteDevice(sameAddr.id)
                }
            }
            merged.values.forEach { store.upsertDevice(it) }
            val answered = found.map { it.id }.toSet()
            _state.update {
                it.copy(
                    scanning = false,
                    online = answered,
                    devices = merged.values.sortedBy { d -> d.name.lowercase() }
                )
            }
        }
    }

    private fun acquireMulticastLock(): WifiManager.MulticastLock? = try {
        val wifi = getApplication<Application>()
            .getSystemService(Context.WIFI_SERVICE) as WifiManager
        wifi.createMulticastLock("multirk").apply {
            setReferenceCounted(false)
            acquire()
        }
    } catch (_: Exception) {
        null
    }

    fun addManual(host: String, portText: String) {
        val h = host.trim()
        if (h.isEmpty()) return
        val port = portText.trim().toIntOrNull() ?: 47820
        val device = Device(
            id = "manual-$h",
            name = h,
            host = h,
            port = port,
            lastSeen = System.currentTimeMillis()
        )
        viewModelScope.launch(Dispatchers.IO) {
            store.upsertDevice(device)
            val all = store.devices()
            _state.update {
                it.copy(
                    devices = all,
                    manualDialog = false,
                    pairDevice = device
                )
            }
        }
    }

    /** Debug/automation hook: add, pair and open in one step. */
    fun quickConnect(host: String, port: Int) {
        val device = Device(
            id = "manual-$host",
            name = host,
            host = host,
            port = port,
            lastSeen = System.currentTimeMillis()
        )
        _state.update { it.copy(busy = true, busyText = "CONECTANDO A $host…") }
        viewModelScope.launch(Dispatchers.IO) {
            var c: MultirkClient? = null
            try {
                c = MultirkClient(host, port)
                c.connect()
                if (!c.authorized) c.pair("Android")
                val realId = c.info.optString("deviceId").ifBlank { device.id }
                val updated = device.copy(
                    id = realId,
                    token = c.token,
                    paired = c.authorized,
                    name = reportedModel(c.info).ifEmpty { c.info.optString("name", host) },
                    lastSeen = System.currentTimeMillis()
                )
                c.close()
                if (realId != device.id) store.deleteDevice(device.id)
                store.upsertDevice(updated)
                val all = store.devices()
                val (o, t) = routeFrom(all, _state.value.originId)
                _state.update {
                    it.copy(
                        devices = all,
                        busy = false,
                        originId = o,
                        targetId = t
                    )
                }
                if (!updated.paired) {
                    toast("CONSOLA NO EMPAREJADA")
                } else {
                    _state.update {
                        it.copy(
                            message = if (all.count { d -> d.paired } > 1) {
                                "DESTINO EMPAREJADO"
                            } else {
                                "CONSOLA EMPAREJADA"
                            }
                        )
                    }
                }
            } catch (e: Exception) {
                _state.update { it.copy(busy = false) }
                toast("ERROR: ${e.message}")
            } finally {
                runCatching { c?.close() }
            }
        }
    }

    // ------------------------------------------------------------ catalog ---

    fun unpair(device: Device) {
        viewModelScope.launch(Dispatchers.IO) {
            store.upsertDevice(device.copy(paired = false, token = null))
            store.clearCatalog(device.id)
            val all = store.devices()
            _state.update { s ->
                val preferred = if (s.originId == device.id) null else s.originId
                val (o, t) = routeFrom(all, preferred)
                s.copy(
                    devices = all,
                    selected = all.firstOrNull { it.id == o },
                    originId = o,
                    targetId = t,
                    screen = if (s.screen == Screen.Catalog && s.selected?.id == device.id) {
                        Screen.History
                    } else {
                        s.screen
                    },
                    message = "${device.name} DESEMPAREJADA"
                )
            }
        }
    }

    fun swapRoute() {
        val s = _state.value
        val origin = s.originId
        val target = s.targetId
        if (origin == null || target == null) {
            toast("AÑADE UNA SEGUNDA CONSOLA")
            return
        }
        val newOrigin = s.devices.firstOrNull { d -> d.id == target } ?: return
        _state.update {
            it.copy(
                originId = target,
                targetId = origin,
                selected = newOrigin,
                games = emptyList(),
                total = 0,
                message = "ORIGEN ⇄ DESTINO"
            )
        }
        openDevice(newOrigin)
    }

    private fun routeFrom(devices: List<Device>, preferredOrigin: String?): Pair<String?, String?> {
        val pair = devices.filter { it.paired }
        val origin = pair.firstOrNull { it.id == preferredOrigin } ?: pair.firstOrNull()
        val target = pair.firstOrNull { it.id != origin?.id }
        return origin?.id to target?.id
    }

    /** Board name reported by the agent ("AISLPC R36TMax" on RK3326). */
    private fun reportedModel(info: JSONObject): String {
        val model = info.optString("model").trim()
        if (model.isNotEmpty() && !model.equals("rk3326", ignoreCase = true)) return model
        val cpu = info.optJSONObject("system")?.optString("cpu")?.trim().orEmpty()
        return if (cpu.isNotEmpty() && !cpu.equals("unknown", ignoreCase = true)) cpu else ""
    }

    /** Names written by older versions or auto-generated IPs get replaced by the model. */
    private fun isAutoName(name: String): Boolean {
        val n = name.trim()
        return n.isEmpty() ||
            n.equals("RG351V", ignoreCase = true) ||
            n.equals("rk3326", ignoreCase = true) ||
            n.startsWith("manual-") ||
            n.matches(Regex("""\d+\.\d+\.\d+\.\d+"""))
    }

    /** One-shot rename of paired consoles that still show a generic name. */
    private fun refreshDeviceNames() {
        viewModelScope.launch(Dispatchers.IO) {
            val paired = store.devices().filter { it.paired && !it.token.isNullOrBlank() }
            var changed = false
            for (d in paired) {
                if (!isAutoName(d.name)) continue
                var c: MultirkClient? = null
                try {
                    c = MultirkClient(d.host, d.port, d.token)
                    c.connect()
                    if (!c.authorized) continue
                    val hw = reportedModel(c.infoFull())
                    if (hw.isNotEmpty() && !hw.equals(d.name, ignoreCase = true)) {
                        store.upsertDevice(d.copy(name = hw, model = hw))
                        changed = true
                    }
                } catch (_: Exception) {
                    // Offline console: keep whatever name it already had.
                } finally {
                    runCatching { c?.close() }
                }
            }
            if (changed) {
                val all = store.devices()
                _state.update { it.copy(devices = all) }
            }
        }
    }

    fun openDeviceInfo(device: Device) {
        _state.update {
            it.copy(
                infoDevice = device,
                infoData = null,
                infoError = null,
                infoLoading = device.paired
            )
        }
        if (!device.paired) return

        viewModelScope.launch(Dispatchers.IO) {
            var c: MultirkClient? = null
            try {
                c = MultirkClient(device.host, device.port, device.token)
                c.connect()
                if (!c.authorized) throw AuthRequiredException()
                val raw = c.infoFull()
                val info = parseDeviceInfo(raw)
                val hw = reportedModel(raw)
                if (hw.isNotEmpty() && !hw.equals(device.name, ignoreCase = true)) {
                    store.upsertDevice(device.copy(name = hw, model = hw))
                }
                _state.update {
                    it.copy(
                        infoData = info,
                        infoLoading = false,
                        infoDevice = it.infoDevice?.copy(
                            name = if (hw.isNotEmpty()) hw else it.infoDevice?.name ?: device.name
                        ),
                        devices = store.devices()
                    )
                }
            } catch (e: Exception) {
                _state.update {
                    it.copy(infoLoading = false, infoError = e.message ?: "ERROR DE CONEXIÓN")
                }
            } finally {
                runCatching { c?.close() }
            }
        }
    }

    fun closeDeviceInfo() {
        _state.update {
            it.copy(infoDevice = null, infoData = null, infoError = null, infoLoading = false)
        }
    }

    /**
     * Destination for [deviceId]'s catalog. One console: never a destination.
     * Two consoles: keep the chosen one, otherwise the other paired console.
     */
    private fun targetFor(deviceId: String, current: String?): String? {
        if (_state.value.consoleMode == 1) return null
        val all = store.devices()
        return current?.takeIf { c -> c != deviceId && all.any { it.id == c && it.paired } }
            ?: all.firstOrNull { it.paired && it.id != deviceId }?.id
    }

    fun openCatalog() {
        val s = _state.value
        val origin = s.devices.firstOrNull { it.id == s.originId }
            ?: s.devices.firstOrNull { it.paired }
        if (origin == null) {
            toast("EMPAREJA UNA CONSOLA PARA VER EL CATÁLOGO")
            return
        }
        openDevice(origin)
    }

    fun openDevice(device: Device) {
        if (!device.paired || device.token.isNullOrBlank()) {
            showAddConsole()
        }
        viewModelScope.launch(Dispatchers.IO) {
            _state.update {
                it.copy(
                    originId = device.id,
                    targetId = targetFor(device.id, it.targetId),
                    selected = device
                )
            }
            if (store.hasCatalog(device.id)) {
                val systems = store.systems(device.id)
                _state.update {
                    it.copy(
                        screen = Screen.Catalog,
                        selected = device,
                        systems = systems,
                        games = emptyList(),
                        filterSystem = null,
                        query = "",
                        onlyState = false,
                        page = 0,
                        total = 0,
                        targetId = targetFor(device.id, it.targetId)
                    )
                }
                loadPage()
                return@launch
            }
            fetchCatalogNow(device, manual = false)
        }
    }

    /** [system] = refresh only that system; null = every system of the console. */
    fun refreshCatalog(system: String? = null) {
        val device = _state.value.selected ?: return
        viewModelScope.launch(Dispatchers.IO) {
            fetchCatalogNow(device, manual = true, system = system)
        }
    }

    private fun fetchCatalogNow(device: Device, manual: Boolean, system: String? = null) {
        val sysName = system?.let { k -> _state.value.systems.firstOrNull { it.key == k }?.name ?: k }
        _state.update {
            it.copy(
                busy = true,
                busyText = when {
                    !manual -> "LEYENDO CATÁLOGO…"
                    system != null -> "ACTUALIZANDO ${sysName?.uppercase()}…"
                    else -> "ACTUALIZANDO TODOS LOS SISTEMAS…"
                }
            )
        }
        var client: MultirkClient? = null
        try {
            client = MultirkClient(device.host, device.port, device.token)
            client.connect()
            if (!client.authorized) {
                _state.update { it.copy(busy = false, pairDevice = device) }
                return
            }

            // A manual refresh makes the console re-read its SD first (only that
            // system when one is chosen), so edits made in ES show up.
            if (manual) client.refresh(system)

            if (manual && system != null) {
                val part = client.catalog(null, system)
                store.replaceSystem(device.id, system, part.systems, part.games)
                val systemsNow = store.systems(device.id)
                val updated = device.copy(
                    systems = systemsNow.size,
                    games = store.countGames(device.id),
                    lastSeen = System.currentTimeMillis()
                )
                store.upsertDevice(updated)
                _state.update {
                    it.copy(
                        busy = false,
                        screen = Screen.Catalog,
                        selected = updated,
                        devices = store.devices(),
                        systems = systemsNow,
                        page = 0,
                        message = "${sysName?.uppercase()} ACTUALIZADO · ${part.games.size} JUEGOS"
                    )
                }
                loadPage()
                return
            }

            val since = if (manual) store.meta(device.id, "catalogStamp") else null
            val data = client.catalog(since)
            if (data.notModified) {
                _state.update { it.copy(busy = false, message = "SIN CAMBIOS · CACHÉ AL DÍA") }
                return
            }

            store.saveSystems(device.id, data.systems)
            store.saveGames(device.id, data.games)
            store.setMeta(device.id, "catalogStamp", data.stamp)

            val updated = device.copy(
                systems = data.systems.size,
                games = data.games.size,
                lastSeen = System.currentTimeMillis()
            )
            store.upsertDevice(updated)

            val s = _state.value
            _state.update {
                it.copy(
                    busy = false,
                    screen = Screen.Catalog,
                    selected = updated,
                    devices = store.devices(),
                    systems = store.systems(device.id),
                    games = emptyList(),
                    filterSystem = s.filterSystem,
                    query = s.query,
                    onlyState = s.onlyState,
                    page = 0,
                    total = 0,
                    message = "CATÁLOGO ACTUALIZADO · ${data.games.size} JUEGOS",
                    targetId = targetFor(updated.id, it.targetId)
                )
            }
            loadPage()
        } catch (e: AuthRequiredException) {
            _state.update { it.copy(busy = false, pairDevice = device) }
        } catch (e: Exception) {
            _state.update { it.copy(busy = false) }
            if (e.isConnectionLoss()) connectionLost(device) else toast(e.message ?: "ERROR")
        } finally {
            runCatching { client?.close() }
        }
    }

    private fun applyFilters(
        games: List<Game>,
        system: String?,
        query: String,
        onlyState: Boolean
    ): List<Game> {
        val q = query.trim()
        return games.filter { g ->
            (system == null || g.system == system) &&
                (!onlyState || g.hasState) &&
                (q.isEmpty() || g.name.contains(q, ignoreCase = true) ||
                    g.file.contains(q, ignoreCase = true))
        }
    }
    private fun loadPage() {
        val s = _state.value
        val device = s.selected ?: return
        viewModelScope.launch(Dispatchers.IO) {
            val total = store.countGames(device.id, s.filterSystem, s.query, s.onlyState)
            val pages = maxOf(1, (total + PAGE_SIZE - 1) / PAGE_SIZE)
            val page = s.page.coerceIn(0, pages - 1)
            val list = store.games(
                device.id,
                s.filterSystem,
                s.query,
                s.onlyState,
                limit = PAGE_SIZE,
                offset = page * PAGE_SIZE
            )
            _state.update { it.copy(games = list, total = total, page = page) }
        }
    }

    private val coverGate = kotlinx.coroutines.sync.Semaphore(2)

    /** Cover from the console, cached on disk so it is requested only once. */
    suspend fun loadCover(game: Game): android.graphics.Bitmap? =
        kotlinx.coroutines.withContext(Dispatchers.IO) {
            val device = _state.value.selected ?: return@withContext null
            if (!game.hasImage) return@withContext null
            val dir = java.io.File(getApplication<Application>().cacheDir, "covers/${device.id}")
            dir.mkdirs()
            val file = java.io.File(dir, game.id.replace(Regex("[^A-Za-z0-9._-]"), "_") + ".jpg")
            if (!file.exists() || file.length() == 0L) {
                coverGate.acquire()
                var client: MultirkClient? = null
                try {
                    client = MultirkClient(device.host, device.port, device.token)
                    client.connect()
                    val bytes = client.media(game.id, "boxart")
                    val tmp = java.io.File(dir, file.name + ".tmp")
                    tmp.writeBytes(bytes)
                    tmp.renameTo(file)
                } catch (e: Exception) {
                    return@withContext null
                } finally {
                    runCatching { client?.close() }
                    coverGate.release()
                }
            }
            android.graphics.BitmapFactory.decodeFile(file.absolutePath)
        }

    /** The console stopped answering: ask the user to reconnect or close the app. */
    private fun connectionLost(device: Device) {
        _state.update {
            if (it.lostDevice != null) it
            else it.copy(
                lostDevice = device, reconnecting = false, reconnectFailed = false,
                busy = false, scraping = false, infoLoading = false
            )
        }
    }

    /** Tries to reach the console again; if it is not ready the dialog stays and the button is free again. */
    fun reconnect() {
        val d = _state.value.lostDevice ?: return
        if (_state.value.reconnecting) return
        _state.update { it.copy(reconnecting = true, reconnectFailed = false) }
        viewModelScope.launch(Dispatchers.IO) {
            var c: MultirkClient? = null
            val ok = try {
                c = MultirkClient(d.host, d.port, d.token)
                c.connect()
                if (!c.authorized) {
                    val tok = c.pair("Android")
                    store.upsertDevice(d.copy(token = tok, paired = true))
                }
                true
            } catch (e: Exception) {
                false
            } finally {
                runCatching { c?.close() }
            }
            _state.update {
                if (ok) it.copy(
                    lostDevice = null, reconnecting = false, reconnectFailed = false,
                    devices = store.devices(), message = "RECONECTADO"
                ) else it.copy(reconnecting = false, reconnectFailed = true)
            }
        }
    }

    /** Scrape on the console, then refresh this game, its cover and the catalog. */
    fun scrapeGame() {
        val s = _state.value
        val device = s.selected ?: return
        val game = s.game ?: return
        if (s.scraping) return
        _state.update { it.copy(scraping = true, scrapingText = "Scrapeando… puede tardar unos segundos") }
        viewModelScope.launch(Dispatchers.IO) {
            var client: MultirkClient? = null
            try {
                client = MultirkClient(device.host, device.port, device.token)
                client.connect()
                val c = s.scraper
                client.scrape(game.id, "es", c.user, c.pass, c.devId, c.devPass, keepImage = game.hasImage)
                refreshAfterChange(client, device, game)
            } catch (e: Exception) {
                _state.update { it.copy(scraping = false) }
                if (e.isConnectionLoss()) connectionLost(device) else toast(e.message ?: "NO SE PUDO SCRAPEAR")
            } finally {
                runCatching { client?.close() }
            }
        }
    }

    /** After the console changed a game: reload it, its cover and the catalog. */
    private fun refreshAfterChange(client: MultirkClient, device: Device, game: Game) {
        val (g, arts) = client.game(game.id)
        val dir = java.io.File(getApplication<Application>().cacheDir, "covers/${device.id}")
        java.io.File(dir, game.id.replace(Regex("[^A-Za-z0-9._-]"), "_") + ".jpg").delete()
        val data = client.catalog(null)
        if (!data.notModified) {
            store.saveSystems(device.id, data.systems)
            store.saveGames(device.id, data.games)
            store.setMeta(device.id, "catalogStamp", data.stamp)
        }
        store.saveArtifacts(device.id, game.id, arts)
        _state.update {
            it.copy(
                scraping = false,
                game = g ?: it.game,
                artifacts = arts,
                coverVersion = it.coverVersion + 1,
                message = "JUEGO ACTUALIZADO"
            )
        }
        // The list on screen was loaded before the change: reload that page so
        // the card gets the new data and its new cover.
        loadPage()
    }

    /**
     * Deletes the game on the console (ROM + media + gamelist entry, done by the
     * agent) and then cleans only that game from the phone: cached row, saves,
     * cover and the page on screen. No full catalogue download.
     */
    fun launchGame() {
        val s = _state.value
        val device = s.selected ?: return
        val game = s.game ?: return
        viewModelScope.launch(Dispatchers.IO) {
            var client: MultirkClient? = null
            try {
                client = MultirkClient(device.host, device.port, device.token)
                client.connect()
                client.launchGame(game.id)
                _state.update { it.copy(playing = game.name) }
            } catch (e: Exception) {
                if (e.isConnectionLoss()) connectionLost(device) else toast(e.message ?: "NO SE PUDO LANZAR")
            } finally {
                runCatching { client?.close() }
            }
        }
    }

    fun stopGame() {
        val device = _state.value.selected ?: return
        _state.update { it.copy(playing = null) }
        viewModelScope.launch(Dispatchers.IO) {
            var client: MultirkClient? = null
            try {
                client = MultirkClient(device.host, device.port, device.token)
                client.connect()
                client.stopGame()
            } catch (e: Exception) {
                if (e.isConnectionLoss()) connectionLost(device) else toast(e.message ?: "NO HAY JUEGO EN MARCHA")
            } finally {
                runCatching { client?.close() }
            }
        }
    }

    fun dismissPlaying() {
        _state.update { it.copy(playing = null) }
    }

    fun deleteGame() {
        val s = _state.value
        val device = s.selected ?: return
        val game = s.game ?: return
        if (s.scraping) return
        _state.update { it.copy(scraping = true, scrapingText = "Eliminando… un momento") }
        viewModelScope.launch(Dispatchers.IO) {
            var client: MultirkClient? = null
            try {
                client = MultirkClient(device.host, device.port, device.token)
                client.connect()
                client.deleteGame(game.id)
                store.removeGame(device.id, game)
                val dir = java.io.File(getApplication<Application>().cacheDir, "covers/${device.id}")
                java.io.File(dir, game.id.replace(Regex("[^A-Za-z0-9._-]"), "_") + ".jpg").delete()
                val systems = store.systems(device.id)
                _state.update {
                    it.copy(
                        scraping = false,
                        screen = Screen.Catalog,
                        game = null,
                        artifacts = emptyList(),
                        systems = systems,
                        coverVersion = it.coverVersion + 1,
                        message = "JUEGO ELIMINADO"
                    )
                }
                loadPage()
            } catch (e: Exception) {
                _state.update { it.copy(scraping = false) }
                if (e.isConnectionLoss()) connectionLost(device) else toast(e.message ?: "NO SE PUDO ELIMINAR")
            } finally {
                runCatching { client?.close() }
            }
        }
    }

    /** Manual info (systems without a scraper): fields + optional picture from the phone. */
    fun saveGameInfo(m: GameMeta, imageUri: android.net.Uri?) {
        val s = _state.value
        val device = s.selected ?: return
        val game = s.game ?: return
        if (s.scraping) return
        _state.update { it.copy(scraping = true, scrapingText = "Guardando… un momento") }
        viewModelScope.launch(Dispatchers.IO) {
            var client: MultirkClient? = null
            try {
                val meta = JSONObject()
                    .put("name", m.name).put("desc", m.desc)
                    .put("developer", m.developer).put("publisher", m.publisher)
                    .put("genre", m.genre).put("players", m.players)
                val year = m.year.trim()
                if (year.length == 4 && year.all { it.isDigit() }) {
                    meta.put("releasedate", year + "0101T000000")
                }
                m.rating.trim().replace(',', '.').toDoubleOrNull()?.let {
                    if (it > 0) meta.put("rating", (it.coerceAtMost(10.0)) / 10.0)
                }
                if (imageUri != null) {
                    meta.put("image", encodeImage(imageUri))
                }
                client = MultirkClient(device.host, device.port, device.token)
                client.connect()
                client.setMeta(game.id, meta)
                refreshAfterChange(client, device, game)
            } catch (e: Exception) {
                _state.update { it.copy(scraping = false) }
                if (e.isConnectionLoss()) connectionLost(device) else toast(e.message ?: "NO SE PUDO GUARDAR")
            } finally {
                runCatching { client?.close() }
            }
        }
    }

    /** Shrinks the picked image (max 800 px, JPEG) so it travels fast. */
    private fun encodeImage(uri: android.net.Uri): String {
        val resolver = getApplication<Application>().contentResolver
        val bounds = android.graphics.BitmapFactory.Options().apply { inJustDecodeBounds = true }
        resolver.openInputStream(uri)?.use { android.graphics.BitmapFactory.decodeStream(it, null, bounds) }
        var sample = 1
        while (bounds.outWidth / sample > 1600 || bounds.outHeight / sample > 1600) sample *= 2
        val opts = android.graphics.BitmapFactory.Options().apply { inSampleSize = sample }
        val bmp = resolver.openInputStream(uri)?.use {
            android.graphics.BitmapFactory.decodeStream(it, null, opts)
        } ?: throw java.io.IOException("no se pudo leer la imagen")
        val scale = 800f / maxOf(bmp.width, bmp.height)
        val out = if (scale < 1f) {
            android.graphics.Bitmap.createScaledBitmap(
                bmp, (bmp.width * scale).toInt().coerceAtLeast(1), (bmp.height * scale).toInt().coerceAtLeast(1), true
            )
        } else bmp
        val bytes = java.io.ByteArrayOutputStream()
        out.compress(android.graphics.Bitmap.CompressFormat.JPEG, 85, bytes)
        return android.util.Base64.encodeToString(bytes.toByteArray(), android.util.Base64.NO_WRAP)
    }

    /**
     * The ficha comes from the catalogue stored on the phone (it already carries
     * the gamelist data). Only the saves are refreshed from the console, in the
     * background, so the screen opens at once and also works offline.
     */
    fun openGame(game: Game) {
        val device = _state.value.selected ?: return
        viewModelScope.launch(Dispatchers.IO) {
            val stored = store.game(device.id, game.id) ?: game
            _state.update {
                it.copy(
                    screen = Screen.Game,
                    game = stored,
                    artifacts = store.artifacts(device.id, game.id),
                    targetId = targetFor(device.id, it.targetId)
                )
            }
            var client: MultirkClient? = null
            try {
                client = MultirkClient(device.host, device.port, device.token)
                client.connect()
                val (_, arts) = client.game(game.id)
                store.saveArtifacts(device.id, game.id, arts)
                _state.update {
                    if (it.game?.id == game.id) it.copy(artifacts = arts) else it
                }
            } catch (e: Exception) {
                if (e.isConnectionLoss()) connectionLost(device) else toast(e.message ?: "NO SE PUDIERON LEER LOS SAVES")
            } finally {
                runCatching { client?.close() }
            }
        }
    }

    // --------------------------------------------------------------- sync ---

    fun sendArtifact(artifact: Artifact) {
        val s = _state.value
        val source = s.devices.firstOrNull { it.id == s.originId } ?: return
        val game = s.game ?: return
        val targetId = s.targetId ?: run {
            toast("AÑADE UNA SEGUNDA CONSOLA")
            return
        }
        val target = s.devices.firstOrNull { it.id == targetId && it.paired } ?: run {
            toast("AÑADE UNA SEGUNDA CONSOLA")
            return
        }
        if (target.id == source.id) {
            toast("ELIGE OTRA CONSOLA")
            return
        }

        _state.update { it.copy(busy = true, busyText = "DESCARGANDO DE ${source.name}…") }
        viewModelScope.launch(Dispatchers.IO) {
            var a: MultirkClient? = null
            var b: MultirkClient? = null
            try {
                a = MultirkClient(source.host, source.port, source.token)
                a.connect()
                val out = ByteArrayOutputStream()
                val meta = a.download(game.id, artifact.kind, artifact.slot, out)
                val bytes = out.toByteArray()

                _state.update { it.copy(busyText = "SUBIENDO A ${target.name}…") }
                b = MultirkClient(target.host, target.port, target.token)
                b.connect()

                var targetGameId = game.id
                runCatching { b.game(game.id) }.getOrNull()?.first?.let { targetGameId = it.id }
                    ?: run {
                        val candidates = b.games(matchKey = game.matchKey, limit = 5).second
                        candidates.firstOrNull()?.let { targetGameId = it.id }
                    }

                b.upload(
                    gameId = targetGameId,
                    kind = artifact.kind,
                    slot = artifact.slot,
                    data = bytes,
                    mtime = meta.mtime
                )

                val event = SyncEvent(
                    ts = System.currentTimeMillis(),
                    game = game.name,
                    from = source.name,
                    to = target.name,
                    kind = artifact.kind,
                    slot = artifact.slot,
                    bytes = bytes.size.toLong(),
                    ok = true,
                    detail = meta.name
                )
                store.log(event)
                _state.update {
                    it.copy(
                        busy = false,
                        message = "SAVE SINCRONIZADO · ${human(bytes.size.toLong())}",
                        log = store.log()
                    )
                }
            } catch (e: Exception) {
                val event = SyncEvent(
                    ts = System.currentTimeMillis(),
                    game = game.name,
                    from = source.name,
                    to = target.name,
                    kind = artifact.kind,
                    slot = artifact.slot,
                    bytes = 0,
                    ok = false,
                    detail = e.message ?: "error"
                )
                store.log(event)
                _state.update {
                    it.copy(
                        busy = false,
                        message = "ERROR: ${e.message ?: "SINCRONIZACIÓN"}",
                        log = store.log()
                    )
                }
            } finally {
                runCatching { a?.close() }
                runCatching { b?.close() }
            }
        }
    }

    // ------------------------------------------------------------ controls ---

    fun selectCtlDevice(device: Device) {
        _state.update { it.copy(ctlDevice = device) }
        loadStatus(device)
    }

    /** Status of the console chosen at the start (the origin). */
    fun loadStatus(device: Device? = _state.value.selected) {
        val d = device ?: return
        _state.update { it.copy(ctlDevice = d) }
        viewModelScope.launch(Dispatchers.IO) {
            var c: MultirkClient? = null
            try {
                c = MultirkClient(d.host, d.port, d.token)
                c.connect()
                val st = c.status()
                _state.update { it.copy(ctl = st) }
            } catch (e: Exception) {
                if (e.isConnectionLoss()) connectionLost(d)
                else _state.update { it.copy(ctl = CtlState(error = e.message)) }
            } finally {
                runCatching { c?.close() }
            }
        }
    }

    fun setVolume(v: Int) {
        val d = _state.value.ctlDevice ?: return
        _state.update { it.copy(ctl = it.ctl.copy(volume = v)) }
        viewModelScope.launch(Dispatchers.IO) {
            var c: MultirkClient? = null
            try {
                c = MultirkClient(d.host, d.port, d.token)
                c.connect()
                val st = c.setVolume(v)
                _state.update { it.copy(ctl = st) }
            } catch (e: Exception) {
                toast("VOLUMEN: ${e.message}")
            } finally {
                runCatching { c?.close() }
            }
        }
    }

    fun setBrightness(v: Int) {
        val d = _state.value.ctlDevice ?: return
        _state.update { it.copy(ctl = it.ctl.copy(brightness = v)) }
        viewModelScope.launch(Dispatchers.IO) {
            var c: MultirkClient? = null
            try {
                c = MultirkClient(d.host, d.port, d.token)
                c.connect()
                val st = c.setBrightness(v)
                _state.update { it.copy(ctl = st) }
            } catch (e: Exception) {
                toast("BRILLO: ${e.message}")
            } finally {
                runCatching { c?.close() }
            }
        }
    }

    fun toggleMute() {
        val d = _state.value.ctlDevice ?: return
        val mute = !(_state.value.ctl.muted ?: false)
        viewModelScope.launch(Dispatchers.IO) {
            var c: MultirkClient? = null
            try {
                c = MultirkClient(d.host, d.port, d.token)
                c.connect()
                val st = c.setMute(mute)
                _state.update { it.copy(ctl = st) }
            } catch (e: Exception) {
                toast("MUTE: ${e.message}")
            } finally {
                runCatching { c?.close() }
            }
        }
    }

    // --------------------------------------------------------------- misc ---

    fun targetDevices(): List<Device> =
        _state.value.devices.filter { it.paired && it.id != _state.value.selected?.id }

    fun setTarget(id: String) {
        _state.update { it.copy(targetId = id) }
    }

    /**
     * Gamelist audit: the console compares every gamelist.xml with its ROMs and
     * drops entries whose file does not exist. Then only the systems that
     * changed are refreshed in the local cache.
     */
    fun auditGamelists() {
        val s = _state.value
        val device = s.selected ?: return
        if (s.scraping) return
        _state.update { it.copy(scraping = true, scrapingText = "Auditando gamelists… puede tardar un poco") }
        viewModelScope.launch(Dispatchers.IO) {
            var client: MultirkClient? = null
            try {
                client = MultirkClient(device.host, device.port, device.token)
                client.connect()
                val r = client.audit()
                val arr = r.optJSONArray("systems")
                val changed = ArrayList<AuditSystem>()
                if (arr != null) {
                    for (i in 0 until arr.length()) {
                        val o = arr.getJSONObject(i)
                        val ents = o.optJSONArray("entries")
                        changed.add(
                            AuditSystem(
                                o.optString("system"), o.optInt("removed"),
                                (0 until (ents?.length() ?: 0)).map { j -> ents!!.getString(j) }
                            )
                        )
                    }
                }
                for (c in changed) {
                    val part = client.catalog(null, c.system)
                    store.replaceSystem(device.id, c.system, part.systems, part.games)
                }
                val systemsNow = store.systems(device.id)
                _state.update {
                    it.copy(
                        scraping = false,
                        systems = systemsNow,
                        auditReport = AuditReport(r.optInt("checked"), r.optInt("removed"), changed)
                    )
                }
                if (changed.isNotEmpty()) loadPage()
            } catch (e: Exception) {
                _state.update { it.copy(scraping = false) }
                if (e.isConnectionLoss()) connectionLost(device) else toast(e.message ?: "NO SE PUDO AUDITAR")
            } finally {
                runCatching { client?.close() }
            }
        }
    }

    /** Long press on a system: hide it (it can be shown again in Ajustes). */
    fun hideSystem(key: String) {
        val device = _state.value.selected ?: return
        viewModelScope.launch(Dispatchers.IO) {
            store.setHidden(device.id, key, true)
            _state.update {
                it.copy(
                    systems = store.systems(device.id),
                    hiddenSystems = store.hiddenSystemInfos(device.id),
                    filterSystem = if (it.filterSystem == key) null else it.filterSystem,
                    page = 0,
                    message = "SISTEMA OCULTO · LO VERÁS EN AJUSTES"
                )
            }
            loadPage()
        }
    }

    fun showSystem(key: String) {
        val device = _state.value.selected ?: return
        viewModelScope.launch(Dispatchers.IO) {
            store.setHidden(device.id, key, false)
            _state.update {
                it.copy(
                    systems = store.systems(device.id),
                    hiddenSystems = store.hiddenSystemInfos(device.id),
                    message = "SISTEMA VISIBLE OTRA VEZ"
                )
            }
            loadPage()
        }
    }

    fun loadHiddenSystems() {
        val device = _state.value.selected ?: return
        viewModelScope.launch(Dispatchers.IO) {
            _state.update { it.copy(hiddenSystems = store.hiddenSystemInfos(device.id)) }
        }
    }

    fun closeAuditReport() {
        _state.update { it.copy(auditReport = null) }
    }

    /** Clears the whole local cache: every catalogue and the downloaded covers. */
    fun clearCache() {
        viewModelScope.launch(Dispatchers.IO) {
            store.devices().forEach { store.clearCatalog(it.id) }
            java.io.File(getApplication<Application>().cacheDir, "covers").deleteRecursively()
            _state.update {
                it.copy(
                    games = emptyList(), systems = emptyList(), total = 0, page = 0,
                    coverVersion = it.coverVersion + 1, message = "CACHÉ LOCAL BORRADA"
                )
            }
        }
    }

    private fun toast(msg: String) {
        _state.update { it.copy(message = msg) }
    }

    private fun human(n: Long): String {
        if (n < 1024) return "$n B"
        val units = arrayOf("KB", "MB", "GB")
        var value = n.toDouble() / 1024
        var i = 0
        while (value >= 1024 && i < units.lastIndex) {
            value /= 1024
            i++
        }
        return String.format("%.1f %s", value, units[i])
    }
}

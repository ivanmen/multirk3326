package org.multirk.app.ui

import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import kotlinx.coroutines.delay
import org.multirk.app.AgentState
import org.multirk.app.AppViewModel
import org.multirk.app.GameMeta
import org.multirk.app.PAGE_SIZE
import org.multirk.app.ScraperCreds
import org.multirk.app.R
import org.multirk.app.Screen
import org.multirk.app.UiState
import org.multirk.app.data.Artifact
import org.multirk.app.data.CtlState
import org.multirk.app.data.Device
import org.multirk.app.data.Game
import org.multirk.app.data.SystemInfo
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import kotlin.math.roundToInt

private val cozyCovers = listOf(
    listOf(Color(0xFFE08A4F), Color(0xFFC8622F)),
    listOf(Color(0xFF8FAF8C), Color(0xFF6F8F72)),
    listOf(Color(0xFFE8B45C), Color(0xFFC98B2E)),
    listOf(Color(0xFFD98C86), Color(0xFFB85A4E)),
    listOf(Color(0xFF9A86B8), Color(0xFF6E5B8C)),
    listOf(Color(0xFF7FA7C4), Color(0xFF4F7896))
)

@Composable
fun MultirkApp(vm: AppViewModel) {
    val s by vm.state.collectAsState()

    CozyWindow()
    MultirkTheme {
        Box(Modifier.fillMaxSize().background(CozyBg)) {
            when {
                s.screen == Screen.History -> HistoryScreen(
                    mode = s.consoleMode,
                    devices = s.devices,
                    online = s.online,
                    scanning = s.scanning,
                    selection = s.selection,
                    onScan = { vm.scan() },
                    onRefresh = { vm.scan(force = true) },
                    onAdd = vm::showAddConsole,
                    onSelect = vm::selectConsole,
                    onInfo = vm::openDeviceInfo,
                    onBack = { vm.navigate(Screen.Start) }
                )
                s.screen == Screen.Start -> StartScreen(onPick = vm::chooseConsoleCount)
                else -> Column(Modifier.fillMaxSize().systemBarsPadding()) {
                    Box(Modifier.weight(1f)) {
                        when (s.screen) {
                            Screen.History, Screen.Start -> Unit
                            Screen.Catalog -> CatalogScreen(s, vm)
                            Screen.Game -> GameScreen(s, vm)
                            Screen.Controls -> ControlsScreen(s, vm)
                            Screen.Settings -> SettingsScreen(s, vm)
                        }
                    }
                    CozyNavBar(
                        screen = s.screen,
                        onNavigate = vm::navigate,
                        onCatalog = vm::openCatalog
                    )
                }
            }

            if (s.busy) CozyBusy(s.busyText)
            s.lostDevice?.let { d ->
                val ctx = androidx.compose.ui.platform.LocalContext.current
                Dialog(
                    onDismissRequest = {},
                    properties = androidx.compose.ui.window.DialogProperties(
                        dismissOnBackPress = false,
                        dismissOnClickOutside = false
                    )
                ) {
                    CozyCard(Modifier.fillMaxWidth()) {
                        CozyText("Consola desconectada", size = 22, color = CozyInk, family = BalooXB)
                        Spacer(Modifier.height(8.dp))
                        CozyText(
                            "Se perdió la conexión con ${d.name} (${d.host}). Comprueba que esté encendida y en la misma WiFi. Cuando esté lista, pulsa Reconectar.",
                            size = 15,
                            color = CozyInkSoft
                        )
                        if (s.reconnectFailed) {
                            Spacer(Modifier.height(8.dp))
                            CozyText(
                                "La consola todavía no responde. Inténtalo de nuevo.",
                                size = 15,
                                color = CozyDanger,
                                family = BalooB
                            )
                        }
                        Spacer(Modifier.height(14.dp))
                        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                            CozyButton(
                                if (s.reconnecting) "Reconectando…" else "Reconectar",
                                { vm.reconnect() },
                                enabled = !s.reconnecting
                            )
                            CozyButton(
                                "Cerrar app",
                                { (ctx as? android.app.Activity)?.finishAffinity() },
                                style = CozyButtonStyle.Ghost
                            )
                        }
                    }
                }
            }
            if (s.scraping) CozyBusy(s.scrapingText)
            s.message?.let { msg -> CozyToast(msg) { vm.clearMessage() } }
            if (s.manualDialog) {
                ManualDialog(
                    onDismiss = { vm.showManualDialog(false) },
                    onAdd = { host, port -> vm.addManual(host, port) }
                )
            }
            if (s.agent.visible) {
                AgentDialog(
                    state = s.agent,
                    onInstall = { host, user, pass -> vm.installAgent(host, user, pass) },
                    onUninstall = { host, user, pass, purge ->
                        vm.uninstallAgent(host, user, pass, purge)
                    },
                    onUnpairOnly = vm::unpairWithoutAgent,
                    onClose = vm::closeAgentDialog
                )
            }
            s.infoDevice?.let { DeviceInfoDialog(s, vm) }
        }
    }
}

// ---------------------------------------------------------------- catalog ---

@Composable
private fun SyncChoice(title: String, text: String, selected: Boolean, onClick: () -> Unit) {
    val shape = RoundedCornerShape(18.dp)
    Column(
        Modifier
            .fillMaxWidth()
            .clip(shape)
            .background(CozyCard)
            .border(if (selected) 3.dp else 2.dp, if (selected) CozyPrimary else CozyCardBorder, shape)
            .clickable { onClick() }
            .padding(16.dp)
    ) {
        CozyText(title, size = 19, color = if (selected) CozyPrimary else CozyInk, family = BalooXB)
        Spacer(Modifier.height(4.dp))
        CozyText(text, size = 14, color = CozyInkSoft)
    }
}

/**
 * Two-console catalogue: only the sync choices (whole console or one system).
 * No game list yet, and nothing is copied yet. The origin is never modified.
 */
@Composable
private fun SyncOptions(s: UiState, origin: Device, target: Device) {
    var full by remember(origin.id, target.id) { mutableStateOf(true) }
    var system by remember(origin.id, target.id) { mutableStateOf<String?>(null) }
    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 14.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        CozyCard(Modifier.fillMaxWidth()) {
            CozyText("Sincronizar", size = 22, color = CozyInk, family = BalooXB)
            Spacer(Modifier.height(4.dp))
            CozyText(
                "${origin.name} → ${target.name}",
                size = 16,
                color = CozyPrimary,
                family = BalooB
            )
            Spacer(Modifier.height(6.dp))
            CozyText(
                "${origin.name} no se modifica: sus ROMs y su media se quedan como están. Solo se completa ${target.name}.",
                size = 14,
                color = CozyInkSoft
            )
        }
        SyncChoice(
            "Sincronización completa",
            "Todos los sistemas de ${origin.name}.",
            full
        ) { full = true }
        SyncChoice(
            "Por sistema",
            "Eliges uno de los sistemas de ${origin.name}.",
            !full
        ) { full = false }
        if (!full) {
            CozySection("Sistema")
            Row(
                Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                s.systems.forEach { sys ->
                    CozyChip(sys.name, system == sys.key) { system = sys.key }
                }
            }
        }
    }
}

@Composable
private fun CatalogScreen(s: UiState, vm: AppViewModel) {
    BackHandler { vm.navigate(Screen.History) }
    val device = s.selected
    if (device == null) {
        Column(Modifier.fillMaxSize()) {
            CozyTopBar(
                title = "Catálogo",
                onBack = { vm.navigate(Screen.History) }
            )
            CozyText(
                "Abre una consola para ver su catálogo.",
                size = 16,
                color = CozyPrimary,
                modifier = Modifier.padding(18.dp)
            )
        }
        return
    }

    Column(Modifier.fillMaxSize()) {
        val target = s.devices.firstOrNull { it.id == s.targetId }
        CozyTopBar(
            title = "Catálogo",
            subtitle = if (s.consoleMode == 2 && target != null) {
                "${device.name} → ${target.name}"
            } else {
                null
            },
            onBack = { vm.navigate(Screen.History) }
        ) {
            if (s.consoleMode == 2 && target != null) {
                Box(
                    Modifier
                        .size(40.dp)
                        .clip(RoundedCornerShape(13.dp))
                        .background(CozyCard)
                        .border(2.dp, CozyCardBorder, RoundedCornerShape(13.dp))
                        .clickable { vm.swapRoute() },
                    contentAlignment = Alignment.Center
                ) {
                    CozyText("⇄", size = 18, color = CozyPrimary, family = BalooXB)
                }
                Spacer(Modifier.width(8.dp))
            }
        }

        if (s.consoleMode == 2 && target != null) {
            SyncOptions(s, device, target)
            return@Column
        }

        Column(Modifier.padding(horizontal = 14.dp)) {
            CozyField(s.query, "Buscar juego o archivo…") {
                vm.setFilter(s.filterSystem, it, s.onlyState)
            }
            if (s.suggestions.isNotEmpty() && s.query.isNotBlank()) {
                Spacer(Modifier.height(6.dp))
                Column(
                    Modifier
                        .fillMaxWidth()
                        .heightIn(max = 170.dp)
                        .clip(RoundedCornerShape(14.dp))
                        .background(CozyCard)
                        .border(2.dp, CozyCardBorder, RoundedCornerShape(14.dp))
                        .verticalScroll(rememberScrollState())
                        .padding(horizontal = 12.dp, vertical = 2.dp)
                ) {
                    s.suggestions.take(5).forEachIndexed { i, g ->
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .clickable { vm.pickSuggestion(g) }
                                .padding(vertical = 5.dp),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = g.name,
                                fontFamily = BalooSB,
                                fontSize = 15.sp,
                                color = CozyInk,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.weight(1f)
                            )
                            Spacer(Modifier.width(8.dp))
                            Text(
                                text = g.system.replaceFirstChar { it.uppercase() },
                                fontFamily = BalooSB,
                                fontSize = 12.sp,
                                color = CozyInkSoft
                            )
                        }
                        if (i < s.suggestions.take(5).lastIndex) {
                            Box(
                                Modifier
                                    .fillMaxWidth()
                                    .height(1.dp)
                                    .background(CozyCardBorder.copy(alpha = 0.5f))
                            )
                        }
                    }
                }
            }
            Row(
                Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState())
                    .padding(vertical = 10.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                CozyChip("Todos", s.filterSystem == null && !s.onlyState) {
                    vm.setFilter(null, s.query, false)
                }
                CozyChip("Con save", s.onlyState) {
                    vm.setFilter(s.filterSystem, s.query, true)
                }
                s.systems.forEach { sys ->
                    SystemChip(
                        sys,
                        s.filterSystem == sys.key,
                        onHide = { vm.hideSystem(sys.key) }
                    ) {
                        vm.setFilter(sys.key, s.query, s.onlyState)
                    }
                }
            }
        }

        var confirmRefresh by remember { mutableStateOf(false) }
        androidx.compose.material3.pulltorefresh.PullToRefreshBox(
            isRefreshing = false,
            onRefresh = { confirmRefresh = true },
            modifier = Modifier.weight(1f)
        ) {
            LazyVerticalGrid(
                columns = GridCells.Fixed(2),
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = 14.dp, vertical = 4.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalArrangement = Arrangement.spacedBy(14.dp)
            ) {
                items(s.games, key = { it.id }) { game ->
                    CozyGameCard(game, vm::loadCover, s.coverVersion) { vm.openGame(game) }
                }
            }
        }
        if (confirmRefresh) {
            val scopeKey = s.filterSystem
            val scopeName = scopeKey?.let { k -> s.systems.firstOrNull { it.key == k }?.name ?: k }
            Dialog(onDismissRequest = { confirmRefresh = false }) {
                CozyCard(Modifier.fillMaxWidth()) {
                    CozyText(
                        if (scopeName == null) "Actualizar todos los sistemas" else "Actualizar $scopeName",
                        size = 22,
                        color = CozyInk,
                        family = BalooXB
                    )
                    Spacer(Modifier.height(8.dp))
                    CozyText(
                        if (scopeName == null) {
                            "Se actualizarán TODOS los sistemas de ${device.name}: la consola vuelve a leer toda su SD y el teléfono descarga el catálogo completo."
                        } else {
                            "Se actualizará solo «$scopeName» de ${device.name}. El resto de sistemas no se toca."
                        },
                        size = 15,
                        color = CozyInkSoft
                    )
                    Spacer(Modifier.height(14.dp))
                    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        CozyButton("Actualizar", {
                            confirmRefresh = false
                            vm.refreshCatalog(scopeKey)
                        })
                        CozyButton(
                            "Cancelar",
                            { confirmRefresh = false },
                            style = CozyButtonStyle.Ghost
                        )
                    }
                }
            }
        }

        val pages = maxOf(1, (s.total + PAGE_SIZE - 1) / PAGE_SIZE)
        Column(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = 4.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            CozyText(
                "Página ${s.page + 1} / $pages · ${s.total} juegos",
                size = 14,
                color = CozyInkSoft,
                align = TextAlign.Center,
                modifier = Modifier.fillMaxWidth()
            )
            Spacer(Modifier.height(6.dp))
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                CozyButton(
                    "◀ Anterior",
                    { vm.setPage(s.page - 1) },
                    modifier = Modifier.weight(1f),
                    style = CozyButtonStyle.Ghost,
                    enabled = s.page > 0
                )
                CozyButton(
                    "Siguiente ▶",
                    { vm.setPage(s.page + 1) },
                    modifier = Modifier.weight(1f),
                    enabled = s.page < pages - 1
                )
            }
        }
    }
}

@OptIn(androidx.compose.foundation.ExperimentalFoundationApi::class)
@Composable
private fun CozyChip(
    text: String,
    active: Boolean,
    onLongClick: (() -> Unit)? = null,
    onClick: () -> Unit
) {
    val shape = RoundedCornerShape(999.dp)
    Box(
        Modifier
            .clip(shape)
            .background(if (active) CozyPrimary else CozyCard)
            .border(2.dp, if (active) CozyPrimary else CozyCardBorder, shape)
            .combinedClickable(onClick = onClick, onLongClick = onLongClick)
            .padding(horizontal = 12.dp, vertical = 6.dp)
    ) {
        Text(
            text = text,
            fontFamily = if (active) BalooXB else BalooSB,
            fontSize = 13.sp,
            color = if (active) Color.White else CozyInkSoft
        )
    }
}

/** A system chip: tap filters, long press shows an "Ocultar" button. */
@Composable
private fun SystemChip(sys: SystemInfo, active: Boolean, onHide: () -> Unit, onClick: () -> Unit) {
    var menu by remember { mutableStateOf(false) }
    Box {
        CozyChip(sys.name, active, onLongClick = { menu = true }, onClick = onClick)
        androidx.compose.material3.DropdownMenu(
            expanded = menu,
            onDismissRequest = { menu = false }
        ) {
            androidx.compose.material3.DropdownMenuItem(
                text = {
                    Text(
                        text = "Ocultar ${sys.name}",
                        fontFamily = BalooXB,
                        fontSize = 15.sp,
                        color = CozyDanger
                    )
                },
                onClick = {
                    menu = false
                    onHide()
                }
            )
        }
    }
}

@Composable
private fun CozyGameCard(
    game: Game,
    loader: suspend (Game) -> android.graphics.Bitmap?,
    version: Int,
    onClick: () -> Unit
) {
    Column(Modifier.clickable { onClick() }) {
        CozyGameCover(game, Modifier.fillMaxWidth().aspectRatio(0.78f), loader, version = version)
        Spacer(Modifier.height(6.dp))
        Text(
            text = game.name,
            fontFamily = BalooXB,
            fontSize = 14.sp,
            color = CozyInk,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis
        )
        CozyText(
            "${game.system.replaceFirstChar { it.uppercase() }} · ${humanSize(game.size)}",
            size = 12,
            color = CozyInkSoft,
            modifier = Modifier.padding(top = 2.dp)
        )
    }
}

@Composable
fun CozyGameCover(
    game: Game,
    modifier: Modifier = Modifier,
    loader: (suspend (Game) -> android.graphics.Bitmap?)? = null,
    fit: androidx.compose.ui.layout.ContentScale = androidx.compose.ui.layout.ContentScale.Crop,
    version: Int = 0
) {
    val pair = cozyCovers[(game.system.hashCode() and 0x7fffffff) % cozyCovers.size]
    val shape = RoundedCornerShape(16.dp)
    var bmp by remember(game.id, version) { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    if (loader != null) {
        LaunchedEffect(game.id, version) { bmp = loader(game)?.asImageBitmap() }
    }
    Box(
        modifier
            .shadow(3.dp, shape)
            .clip(shape)
            .background(Brush.linearGradient(pair))
            .border(2.dp, CozyCardBorder, shape)
    ) {
        Text(
            text = game.name.take(1).uppercase(),
            fontFamily = BalooXB,
            fontSize = 54.sp,
            color = Color.White.copy(alpha = 0.28f),
            modifier = Modifier.align(Alignment.Center)
        )
        bmp?.let {
            androidx.compose.foundation.Image(
                bitmap = it,
                contentDescription = null,
                contentScale = fit,
                modifier = Modifier.matchParentSize()
            )
        }
        Box(
            Modifier
                .align(Alignment.TopStart)
                .padding(7.dp)
                .clip(RoundedCornerShape(8.dp))
                .background(CozyCard.copy(alpha = 0.95f))
                .padding(horizontal = 7.dp, vertical = 2.dp)
        ) {
            Text(
                text = game.system.replaceFirstChar { it.uppercase() },
                fontFamily = BalooB,
                fontSize = 11.sp,
                color = CozyInk
            )
        }
        if (game.hasState) {
            Box(
                Modifier
                    .align(Alignment.TopEnd)
                    .padding(7.dp)
                    .size(24.dp)
                    .clip(RoundedCornerShape(8.dp))
                    .background(CozySecondary)
                    .border(2.dp, Color.White, RoundedCornerShape(8.dp)),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = "S",
                    fontFamily = BalooXB,
                    fontSize = 11.sp,
                    color = Color.White
                )
            }
        }
    }
}

// ------------------------------------------------------------------- game ---

@Composable
private fun GameScreen(s: UiState, vm: AppViewModel) {
    BackHandler { vm.navigate(Screen.Catalog) }
    val game = s.game ?: return
    val device = s.selected ?: return
    val target = s.devices.firstOrNull { it.id == s.targetId }
    val canSend = target != null
    var editingInfo by remember(game.id) { mutableStateOf(false) }
    var confirmDelete by remember(game.id) { mutableStateOf(false) }
    var confirmPlay by remember(game.id) { mutableStateOf(false) }

    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
    ) {
        Box {
            CozyGameCover(game, Modifier.fillMaxWidth().height(240.dp), vm::loadCover, version = s.coverVersion)
            Box(
                Modifier
                    .align(Alignment.BottomStart)
                    .padding(12.dp)
                    .size(40.dp)
                    .clip(RoundedCornerShape(13.dp))
                    .background(CozyCard)
                    .border(2.dp, CozyCardBorder, RoundedCornerShape(13.dp))
                    .clickable { vm.navigate(Screen.Catalog) },
                contentAlignment = Alignment.Center
            ) {
                CozyText("<", size = 18, color = CozyInk, family = BalooXB)
            }
        }

        Column(Modifier.padding(16.dp)) {
            CozyText(
                "${game.system} · ${humanSize(game.size)}",
                size = 13,
                color = CozyPrimary,
                family = BalooB
            )
            Spacer(Modifier.height(4.dp))
            Text(
                text = game.name,
                fontFamily = BalooXB,
                fontSize = 24.sp,
                color = CozyInk,
                lineHeight = 28.sp
            )
            Spacer(Modifier.height(4.dp))
            CozyText(game.file, size = 13, color = CozyInkSoft)

            if (game.desc.isBlank()) {
                Spacer(Modifier.height(12.dp))
                if (game.scrapable) {
                    CozyButton(
                        if (s.scraping) "Scrapeando…" else "Scrapear y actualizar",
                        { vm.scrapeGame() },
                        enabled = !s.scraping
                    )
                } else {
                    CozyButton("Añadir información", { editingInfo = true }, enabled = !s.scraping)
                }
            }
            if (editingInfo) {
                GameInfoDialog(
                    game = game,
                    onDismiss = { editingInfo = false },
                    onSave = { meta, uri ->
                        editingInfo = false
                        vm.saveGameInfo(meta, uri)
                    }
                )
            }

            val facts = listOf(
                "Desarrollador" to game.developer,
                "Editor" to game.publisher,
                "Género" to game.genre,
                "Jugadores" to game.players,
                "Lanzamiento" to game.releasedate.take(4).takeIf { it.length == 4 }.orEmpty(),
                "Valoración" to if (game.rating > 0) "${(game.rating * 10).toInt()}/10" else ""
            ).filter { it.second.isNotBlank() }
            if (facts.isNotEmpty() || game.desc.isNotBlank()) {
                Spacer(Modifier.height(12.dp))
                CozyCard(Modifier.fillMaxWidth()) {
                    facts.forEach { (k, v) ->
                        Row(Modifier.padding(bottom = 4.dp)) {
                            CozyText("$k: ", size = 14, color = CozyInkSoft, family = BalooB)
                            CozyText(v, size = 14, color = CozyInk)
                        }
                    }
                    if (game.desc.isNotBlank()) {
                        if (facts.isNotEmpty()) Spacer(Modifier.height(6.dp))
                        CozyText(game.desc.trim(), size = 14, color = CozyInk)
                    }
                }
            }

            CozySection("Saves en ${device.name}")

            if (!canSend) {
                CozyCard(Modifier.fillMaxWidth().padding(bottom = 10.dp)) {
                    CozyText("Sincronización desactivada", size = 15, color = CozyPrimary, family = BalooXB)
                    Spacer(Modifier.height(4.dp))
                    CozyText(
                        "Añade una segunda consola como destino para enviar saves.",
                        size = 14,
                        color = CozyInkSoft
                    )
                }
            }
            if (s.artifacts.isEmpty()) {
                CozyCard(Modifier.fillMaxWidth()) {
                    CozyText(
                        "Este juego no tiene estados guardados todavía.",
                        size = 15,
                        color = CozyInkSoft
                    )
                }
            } else {
                s.artifacts.forEach { artifact ->
                    CozyArtifactRow(artifact, enabled = canSend) {
                        vm.sendArtifact(artifact)
                    }
                    Spacer(Modifier.height(10.dp))
                }
            }

            CozySection("Destino")
            CozyCard(Modifier.fillMaxWidth()) {
                CozyText(
                    "${device.name}  →  ${target?.name ?: "falta"}",
                    size = 15,
                    color = if (canSend) CozyPrimary else CozyInkSoft,
                    family = BalooB
                )
                if (!canSend) {
                    Spacer(Modifier.height(4.dp))
                    CozyText(
                        "Para sincronizar, elige el modo «Dos consolas» al abrir la app.",
                        size = 13,
                        color = CozyInkSoft
                    )
                }
            }

            Spacer(Modifier.height(14.dp))
            CozyText(
                "El móvil descarga de la consola origen y sube a la destino. La copia anterior queda como .multirk.bak.",
                size = 12,
                color = CozyPlaceholder
            )
            Spacer(Modifier.height(10.dp))

            CozySection("Jugar")
            CozyButton("Jugar en ${device.name}", { confirmPlay = true })
            if (confirmPlay) {
                Dialog(onDismissRequest = { confirmPlay = false }) {
                    CozyCard(Modifier.fillMaxWidth()) {
                        CozyText("Jugar", size = 22, color = CozyInk, family = BalooXB)
                        Spacer(Modifier.height(8.dp))
                        CozyText(
                            "¿Lanzar «${game.name}» en ${device.name}?",
                            size = 16,
                            color = CozyInk,
                            family = BalooB
                        )
                        Spacer(Modifier.height(6.dp))
                        CozyText(
                            "La consola cierra EmulationStation y abre el juego en su pantalla. Si alguien está jugando, se corta esa partida.",
                            size = 14,
                            color = CozyInkSoft
                        )
                        Spacer(Modifier.height(14.dp))
                        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                            CozyButton("Jugar", {
                                confirmPlay = false
                                vm.launchGame()
                            })
                            CozyButton("Cancelar", { confirmPlay = false }, style = CozyButtonStyle.Ghost)
                        }
                    }
                }
            }
            if (s.playing != null) {
                Dialog(onDismissRequest = { vm.dismissPlaying() }) {
                    CozyCard(Modifier.fillMaxWidth()) {
                        CozyText("Jugando", size = 22, color = CozyInk, family = BalooXB)
                        Spacer(Modifier.height(8.dp))
                        CozyText(s.playing ?: "", size = 16, color = CozyInk, family = BalooB)
                        Spacer(Modifier.height(14.dp))
                        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                            CozyButton("Cerrar juego", { vm.stopGame() }, style = CozyButtonStyle.Danger)
                            CozyButton("Seguir", { vm.dismissPlaying() }, style = CozyButtonStyle.Ghost)
                        }
                    }
                }
            }

            CozySection("Eliminar")
            CozyButton(
                "Eliminar juego",
                { confirmDelete = true },
                style = CozyButtonStyle.GhostDanger,
                enabled = !s.scraping
            )
            if (confirmDelete) {
                Dialog(onDismissRequest = { confirmDelete = false }) {
                    CozyCard(Modifier.fillMaxWidth()) {
                        CozyText("Eliminar juego", size = 22, color = CozyInk, family = BalooXB)
                        Spacer(Modifier.height(8.dp))
                        CozyText(
                            "¿Eliminar «${game.name}» de ${device.name}?",
                            size = 16,
                            color = CozyInk,
                            family = BalooB
                        )
                        Spacer(Modifier.height(6.dp))
                        CozyText(
                            "Se borra la ROM y su imagen, miniatura y vídeo de la SD de la consola, y se quita del gamelist. No se puede deshacer. Los saves no se tocan.",
                            size = 14,
                            color = CozyInkSoft
                        )
                        Spacer(Modifier.height(14.dp))
                        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                            CozyButton("Eliminar", {
                                confirmDelete = false
                                vm.deleteGame()
                            }, style = CozyButtonStyle.Danger)
                            CozyButton(
                                "Cancelar",
                                { confirmDelete = false },
                                style = CozyButtonStyle.Ghost
                            )
                        }
                    }
                }
            }
            Spacer(Modifier.height(10.dp))
        }
    }
}

@Composable
private fun CozyArtifactRow(artifact: Artifact, enabled: Boolean, onSend: () -> Unit) {
    CozyCard(Modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier
                    .size(40.dp)
                    .clip(RoundedCornerShape(12.dp))
                    .background(CozySecondary.copy(alpha = 0.15f))
                    .border(2.dp, CozySecondary, RoundedCornerShape(12.dp)),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = slotBadge(artifact),
                    fontFamily = BalooXB,
                    fontSize = 14.sp,
                    color = CozySecondary
                )
            }
            Spacer(Modifier.width(10.dp))
            Column(Modifier.weight(1f)) {
                CozyText(artifactLabel(artifact), size = 15, color = CozyInk, family = BalooB)
                CozyText(
                    "${dateLabel(artifact.mtime)} · ${humanSize(artifact.size)}",
                    size = 13,
                    color = CozyInkSoft
                )
            }
            Spacer(Modifier.width(10.dp))
            CozyButton(
                "Enviar",
                onSend,
                modifier = Modifier.width(96.dp),
                style = CozyButtonStyle.Secondary,
                enabled = enabled
            )
        }
    }
}

// --------------------------------------------------------------- controls ---

@Composable
private fun ControlsScreen(s: UiState, vm: AppViewModel) {
    BackHandler { vm.navigate(Screen.Catalog) }
    // The console to control is the one chosen at the start (the origin).
    val device = s.selected
    LaunchedEffect(device?.id) { vm.loadStatus(device) }

    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 18.dp, vertical = 16.dp)
    ) {
        CozyText("Controles", size = 27, color = CozyInk, family = BalooXB)
        CozyText("Volumen · brillo · batería", size = 14, color = CozyInkSoft)
        Spacer(Modifier.height(12.dp))

        if (device == null) {
            CozyCard(Modifier.fillMaxWidth()) {
                CozyText(
                    "Elige una consola para controlar su audio y brillo.",
                    size = 15,
                    color = CozyInkSoft
                )
            }
            return@Column
        }

        CozyVolumeCard(s.ctl, vm)
        Spacer(Modifier.height(12.dp))
        CozyBrightnessCard(s.ctl, vm)
        Spacer(Modifier.height(12.dp))
        CozyBatteryCard(s.ctl)

        Spacer(Modifier.height(12.dp))
        CozyText(
            "El volumen usa amixer o pactl. El brillo escribe en /sys/class/backlight; si falla, instala brightnessctl o añade el usuario al grupo video.",
            size = 12,
            color = CozyPlaceholder
        )
        s.ctl.error?.let {
            Spacer(Modifier.height(6.dp))
            CozyText("Aviso: $it", size = 13, color = CozyDanger)
        }
        Spacer(Modifier.height(8.dp))
    }
}

@Composable
private fun CozyVolumeCard(ctl: CtlState, vm: AppViewModel) {
    var value by remember(ctl.volume) { mutableStateOf((ctl.volume ?: 50).toFloat()) }
    CozyCard(Modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            CozyIcon(R.drawable.cozy_volume, 34.dp)
            Spacer(Modifier.width(10.dp))
            CozyText("Volumen", size = 18, color = CozyInk, family = BalooXB)
            Spacer(Modifier.weight(1f))
            CozyText("${value.roundToInt()}%", size = 20, color = CozyPrimary, family = BalooXB)
        }
        Spacer(Modifier.height(8.dp))
        CozySlider(
            value = value / 100f,
            onChange = { value = it * 100f },
            onFinished = { vm.setVolume(value.roundToInt()) }
        )
        Spacer(Modifier.height(8.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            CozyButton("-5", {
                value = (value - 5).coerceIn(0f, 100f)
                vm.setVolume(value.roundToInt())
            }, modifier = Modifier.weight(1f), style = CozyButtonStyle.Ghost)
            CozyButton("+5", {
                value = (value + 5).coerceIn(0f, 100f)
                vm.setVolume(value.roundToInt())
            }, modifier = Modifier.weight(1f), style = CozyButtonStyle.Ghost)
            CozyButton(
                if (ctl.muted == true) "Mute ON" else "Mute",
                { vm.toggleMute() },
                modifier = Modifier.weight(1f),
                style = if (ctl.muted == true) CozyButtonStyle.Danger else CozyButtonStyle.Ghost
            )
        }
        Spacer(Modifier.height(4.dp))
        CozyText(ctl.audio?.let { "amixer · $it" } ?: "amixer / pactl", size = 12, color = CozyPlaceholder)
    }
}

@Composable
private fun CozyBrightnessCard(ctl: CtlState, vm: AppViewModel) {
    var value by remember(ctl.brightness) { mutableStateOf((ctl.brightness ?: 70).toFloat()) }
    CozyCard(Modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            CozyIcon(R.drawable.cozy_nav_controls, 34.dp)
            Spacer(Modifier.width(10.dp))
            CozyText("Brillo", size = 18, color = CozyInk, family = BalooXB)
            Spacer(Modifier.weight(1f))
            CozyText("${value.roundToInt()}%", size = 20, color = CozySecondary, family = BalooXB)
        }
        Spacer(Modifier.height(8.dp))
        CozySlider(
            value = value / 100f,
            onChange = { value = it * 100f },
            onFinished = { vm.setBrightness(value.roundToInt()) }
        )
        Spacer(Modifier.height(8.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            CozyButton("-5", {
                value = (value - 5).coerceIn(5f, 100f)
                vm.setBrightness(value.roundToInt())
            }, modifier = Modifier.weight(1f), style = CozyButtonStyle.Ghost)
            CozyButton("+5", {
                value = (value + 5).coerceIn(5f, 100f)
                vm.setBrightness(value.roundToInt())
            }, modifier = Modifier.weight(1f), style = CozyButtonStyle.Ghost)
            CozyButton("Bajo", {
                value = 25f
                vm.setBrightness(25)
            }, modifier = Modifier.weight(1f), style = CozyButtonStyle.Ghost)
        }
        Spacer(Modifier.height(4.dp))
        CozyText(
            ctl.backlight?.let { "sysfs · $it" } ?: "sysfs / brightnessctl",
            size = 12,
            color = CozyPlaceholder
        )
    }
}

@Composable
private fun CozyBatteryCard(ctl: CtlState) {
    CozyCard(Modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            CozyIcon(R.drawable.cozy_battery, 44.dp)
            Spacer(Modifier.width(10.dp))
            Column {
                CozyText("Batería", size = 18, color = CozyInk, family = BalooXB)
                CozyText(
                    if (ctl.charging == true) "Cargando" else "Descargando",
                    size = 13,
                    color = CozyInkSoft
                )
            }
            Spacer(Modifier.weight(1f))
            CozyText(
                ctl.battery?.let { "$it%" } ?: "—",
                size = 22,
                color = CozySecondary,
                family = BalooXB
            )
        }
    }
}

// --------------------------------------------------------------- settings ---

@Composable
private fun SettingsScreen(s: UiState, vm: AppViewModel) {
    BackHandler { vm.navigate(Screen.Catalog) }
    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 18.dp, vertical = 16.dp)
    ) {
        CozyText("Ajustes", size = 27, color = CozyInk, family = BalooXB)
        CozyText("MULTIRK 0.1", size = 14, color = CozyInkSoft)
        Spacer(Modifier.height(12.dp))

        LaunchedEffect(s.selected?.id) { vm.loadHiddenSystems() }
        var showHidden by remember { mutableStateOf(false) }
        CozySection("Sistemas ocultos")
        CozyCard(Modifier.fillMaxWidth()) {
            CozyText(
                "Los sistemas que ocultas en el catálogo (manteniéndolos pulsados) no salen en la app. Aquí puedes verlos y hacerlos reaparecer.",
                size = 14,
                color = CozyInkSoft
            )
            Spacer(Modifier.height(10.dp))
            CozyButton(
                if (s.hiddenSystems.isEmpty()) "Sistemas ocultos" else "Sistemas ocultos (${s.hiddenSystems.size})",
                { showHidden = true },
                style = CozyButtonStyle.Secondary
            )
        }
        if (showHidden) {
            Dialog(onDismissRequest = { showHidden = false }) {
                CozyCard(Modifier.fillMaxWidth()) {
                    CozyText("Sistemas ocultos", size = 22, color = CozyInk, family = BalooXB)
                    Spacer(Modifier.height(8.dp))
                    if (s.hiddenSystems.isEmpty()) {
                        CozyText(
                            "No hay sistemas ocultos. Mantén pulsado un sistema en el catálogo para ocultarlo.",
                            size = 15,
                            color = CozyInkSoft
                        )
                    } else {
                        CozyText(
                            "Pulsa Mostrar para que reaparezcan en el catálogo.",
                            size = 14,
                            color = CozyInkSoft
                        )
                        Spacer(Modifier.height(6.dp))
                        Column(
                            Modifier
                                .heightIn(max = 360.dp)
                                .verticalScroll(rememberScrollState())
                        ) {
                            s.hiddenSystems.forEach { sys ->
                                Row(
                                    Modifier.padding(vertical = 6.dp),
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Column(Modifier.weight(1f)) {
                                        CozyText(sys.name, size = 16, color = CozyInk, family = BalooB)
                                        CozyText(
                                            "${sys.games} " + if (sys.games == 1) "juego" else "juegos",
                                            size = 12,
                                            color = CozyPlaceholder
                                        )
                                    }
                                    Spacer(Modifier.width(10.dp))
                                    Box(Modifier.width(130.dp)) {
                                        CozyButton(
                                            "Mostrar",
                                            { vm.showSystem(sys.key) },
                                            style = CozyButtonStyle.Secondary
                                        )
                                    }
                                }
                            }
                        }
                    }
                    Spacer(Modifier.height(14.dp))
                    CozyButton("Cerrar", { showHidden = false }, style = CozyButtonStyle.Ghost)
                }
            }
        }

        var confirmAudit by remember { mutableStateOf(false) }
        CozySection("Auditoría de gamelists")
        CozyCard(Modifier.fillMaxWidth()) {
            CozyText(
                "Compara cada gamelist.xml de la consola con sus ROMs y quita las entradas cuyo archivo ya no existe. No toca ROMs ni imágenes; cada gamelist se guarda antes con un backup.",
                size = 14,
                color = CozyInkSoft
            )
            Spacer(Modifier.height(10.dp))
            CozyButton(
                "Auditar gamelists",
                { confirmAudit = true },
                enabled = s.selected != null && !s.scraping
            )
        }
        if (confirmAudit) {
            Dialog(onDismissRequest = { confirmAudit = false }) {
                CozyCard(Modifier.fillMaxWidth()) {
                    CozyText("Auditar gamelists", size = 22, color = CozyInk, family = BalooXB)
                    Spacer(Modifier.height(8.dp))
                    CozyText(
                        "Se revisarán los gamelist.xml de ${s.selected?.name ?: "la consola"} y se quitarán las entradas sin ROM. Se hace backup de cada gamelist antes de cambiarlo.",
                        size = 15,
                        color = CozyInkSoft
                    )
                    Spacer(Modifier.height(14.dp))
                    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        CozyButton("Auditar", {
                            confirmAudit = false
                            vm.auditGamelists()
                        })
                        CozyButton(
                            "Cancelar",
                            { confirmAudit = false },
                            style = CozyButtonStyle.Ghost
                        )
                    }
                }
            }
        }
        s.auditReport?.let { rep ->
            Dialog(onDismissRequest = { vm.closeAuditReport() }) {
                CozyCard(Modifier.fillMaxWidth()) {
                    CozyText("Auditoría terminada", size = 22, color = CozyInk, family = BalooXB)
                    Spacer(Modifier.height(8.dp))
                    CozyText(
                        if (rep.removed == 0) {
                            "Se revisaron ${rep.checked} entradas y todas tienen su ROM. No se cambió nada."
                        } else {
                            "Se revisaron ${rep.checked} entradas y se limpiaron ${rep.removed} sin ROM."
                        },
                        size = 15,
                        color = CozyInk,
                        family = BalooB
                    )
                    if (rep.systems.isNotEmpty()) {
                        Spacer(Modifier.height(8.dp))
                        Column(
                            Modifier
                                .heightIn(max = 220.dp)
                                .verticalScroll(rememberScrollState())
                        ) {
                            rep.systems.forEach { sy ->
                                CozyText(
                                    "${sy.system}: ${sy.removed} " + if (sy.removed == 1) "entrada" else "entradas",
                                    size = 15,
                                    color = CozyPrimary,
                                    family = BalooB
                                )
                                sy.entries.forEach { e ->
                                    CozyText("• ${e.removePrefix("./")}", size = 13, color = CozyInkSoft)
                                }
                                Spacer(Modifier.height(6.dp))
                            }
                        }
                    }
                    Spacer(Modifier.height(14.dp))
                    CozyButton("Cerrar", { vm.closeAuditReport() }, style = CozyButtonStyle.Ghost)
                }
            }
        }

        CozySection("Caché local")
        CozyCard(Modifier.fillMaxWidth()) {
            CozyText(
                "Borra el catálogo guardado y las portadas descargadas de este teléfono. Se vuelven a descargar al abrir la consola. No se toca nada de la consola.",
                size = 14,
                color = CozyInkSoft
            )
            Spacer(Modifier.height(10.dp))
            CozyButton(
                "Borrar caché local",
                vm::clearCache,
                style = CozyButtonStyle.GhostDanger
            )
        }

        Spacer(Modifier.height(20.dp))
        CozyText(
            "RK3326 · ArkOS · RetroArch",
            size = 12,
            color = CozyPlaceholder,
            align = TextAlign.Center,
            modifier = Modifier.fillMaxWidth()
        )
        Spacer(Modifier.height(10.dp))
    }
}

// ---------------------------------------------------------------- dialogs ---

@Composable
private fun GameInfoDialog(
    game: Game,
    onDismiss: () -> Unit,
    onSave: (GameMeta, android.net.Uri?) -> Unit
) {
    var name by remember { mutableStateOf(game.name) }
    var desc by remember { mutableStateOf("") }
    var developer by remember { mutableStateOf("") }
    var publisher by remember { mutableStateOf("") }
    var genre by remember { mutableStateOf("") }
    var players by remember { mutableStateOf("") }
    var year by remember { mutableStateOf("") }
    var rating by remember { mutableStateOf("") }
    var image by remember { mutableStateOf<android.net.Uri?>(null) }
    val picker = rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.GetContent()
    ) { uri -> if (uri != null) image = uri }

    Dialog(onDismissRequest = onDismiss) {
        CozyCard(Modifier.fillMaxWidth()) {
            Column(Modifier.verticalScroll(rememberScrollState())) {
                CozyText("Información del juego", size = 22, color = CozyInk, family = BalooXB)
                Spacer(Modifier.height(4.dp))
                CozyText(
                    "Este sistema no tiene scraper. Rellena lo que quieras; se guarda en el gamelist de la consola.",
                    size = 14,
                    color = CozyInkSoft
                )
                Spacer(Modifier.height(12.dp))
                CozyButton(
                    if (image == null) "Elegir imagen" else "Imagen elegida · cambiar",
                    { picker.launch("image/*") },
                    style = CozyButtonStyle.Ghost
                )
                Spacer(Modifier.height(10.dp))
                CozyField(name, "Nombre") { name = it }
                Spacer(Modifier.height(8.dp))
                CozyField(desc, "Descripción", lines = 4) { desc = it }
                Spacer(Modifier.height(8.dp))
                CozyField(developer, "Desarrollador") { developer = it }
                Spacer(Modifier.height(8.dp))
                CozyField(publisher, "Editor") { publisher = it }
                Spacer(Modifier.height(8.dp))
                CozyField(genre, "Género") { genre = it }
                Spacer(Modifier.height(8.dp))
                CozyField(players, "Jugadores (ej. 1-2)") { players = it }
                Spacer(Modifier.height(8.dp))
                CozyField(year, "Año (ej. 1999)", numeric = true) {
                    if (it.length <= 4 && it.all(Char::isDigit)) year = it
                }
                Spacer(Modifier.height(8.dp))
                CozyField(rating, "Valoración 0-10") { rating = it }
                Spacer(Modifier.height(14.dp))
                Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    CozyButton("Guardar", {
                        onSave(
                            GameMeta(
                                name = if (name.trim() == game.name) "" else name,
                                desc = desc, developer = developer, publisher = publisher,
                                genre = genre, players = players, year = year, rating = rating
                            ),
                            image
                        )
                    })
                    CozyButton("Cancelar", onDismiss, style = CozyButtonStyle.Ghost)
                }
            }
        }
    }
}

@Composable
private fun DeviceInfoDialog(s: UiState, vm: AppViewModel) {
    val device = s.infoDevice ?: return
    val role = when {
        !device.paired -> "Sin emparejar"
        device.id == s.originId -> "Origen"
        device.id == s.targetId -> "Destino"
        else -> "Sobra"
    }
    val roleColor = when (role) {
        "Origen" -> CozySecondary
        "Destino" -> CozyPrimary
        "Sobra" -> CozyDanger
        else -> CozyPlaceholder
    }

    Dialog(onDismissRequest = vm::closeDeviceInfo) {
        CozyCard(Modifier.fillMaxWidth()) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                CozyIcon(R.drawable.cozy_logo, 40.dp)
                Spacer(Modifier.width(10.dp))
                Column(Modifier.weight(1f)) {
                    CozyText(device.name, size = 19, color = CozyInk, family = BalooXB)
                    CozyText("${device.host}:${device.port}", size = 14, color = CozyInkSoft)
                }
                CozyPill(role, roleColor, filled = role != "Sin emparejar")
            }

            Spacer(Modifier.height(8.dp))

            if (s.infoLoading) {
                CozyText("Leyendo la consola…", size = 15, color = CozyPrimary)
            }
            s.infoError?.let {
                CozyText("Error: $it", size = 14, color = CozyDanger)
            }

            val info = s.infoData
            if (info != null) {
                Column(
                    Modifier
                        .fillMaxWidth()
                        .heightIn(max = 250.dp)
                        .verticalScroll(rememberScrollState())
                ) {
                    CozySection("Sistema")
                    CozyInfoLine("CPU", info.sys.cpu.ifBlank { "—" })
                    CozyInfoLine("Núcleos", info.sys.cores.toString())
                    CozyInfoLine("Carga 1m", String.format("%.2f", info.sys.load1))
                    CozyInfoLine(
                        "Temp",
                        if (info.sys.tempC > 0) String.format("%.1f °C", info.sys.tempC) else "—"
                    )
                    CozyInfoLine("RAM", "${info.sys.memAvailMB} / ${info.sys.memTotalMB} MB")
                    CozyInfoLine(
                        "SD libre",
                        String.format("%.1f / %.1f GB", info.sys.diskFreeGB, info.sys.diskTotalGB)
                    )
                    CozyInfoLine("Kernel", info.sys.kernel.ifBlank { "—" })
                    CozyInfoLine("Uptime", uptimeLabel(info.sys.uptimeSec))
                    CozyInfoLine("Modelo", info.model.ifBlank { "—" })
                    CozyInfoLine("OS", info.os.ifBlank { "—" })
                    CozyInfoLine("Versión", info.version.ifBlank { "—" })
                    CozyInfoLine("Protocolo", info.protocol.uppercase())

                    CozySection("Catálogo")
                    CozyInfoLine("Juegos", info.games.toString())
                    CozyInfoLine("Sistemas", info.systems.toString())
                    CozyInfoLine("Con save", info.withState.toString())
                }
            } else if (!device.paired) {
                CozyText(
                    "Consola sin emparejar. Pulsa emparejar para añadirla.",
                    size = 15,
                    color = CozyPrimary
                )
            }

            Spacer(Modifier.height(12.dp))
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                if (device.paired) {
                    CozyButton(
                        "Desemparejar",
                        { vm.startUnpair(device) },
                        style = CozyButtonStyle.Danger
                    )
                } else {
                    CozyButton(
                        "Emparejar",
                        {
                            vm.closeDeviceInfo()
                            vm.showPairDialog(device)
                        }
                    )
                }
                CozyButton(
                    "Cerrar",
                    vm::closeDeviceInfo,
                    style = CozyButtonStyle.Ghost
                )
            }
        }
    }
}

@Composable
private fun CozyInfoLine(label: String, value: String) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 3.dp),
        verticalAlignment = Alignment.Top
    ) {
        CozyText(label, size = 14, color = CozyInkSoft, modifier = Modifier.width(120.dp))
        Text(
            text = value,
            fontFamily = BalooSB,
            fontSize = 15.sp,
            color = CozyInk,
            textAlign = TextAlign.End,
            modifier = Modifier.weight(1f)
        )
    }
}

private fun uptimeLabel(sec: Long): String {
    if (sec <= 0) return "—"
    val d = sec / 86400
    val h = (sec % 86400) / 3600
    val m = (sec % 3600) / 60
    return when {
        d > 0 -> "${d}d ${h}h ${m}m"
        h > 0 -> "${h}h ${m}m"
        else -> "${m}m"
    }
}

@Composable
private fun AgentDialog(
    state: AgentState,
    onInstall: (String, String, String) -> Unit,
    onUninstall: (String, String, String, Boolean) -> Unit,
    onUnpairOnly: () -> Unit,
    onClose: () -> Unit
) {
    var host by remember { mutableStateOf(if (state.host.isNotBlank()) state.host else "192.168.1.") }
    var user by remember { mutableStateOf("ark") }
    var pass by remember { mutableStateOf("ark") }
    var purge by remember { mutableStateOf(false) }
    val unpair = state.mode == "unpair"
    val removing = state.mode == "uninstall" || unpair

    Dialog(onDismissRequest = { if (!state.running) onClose() }) {
        CozyCard(Modifier.fillMaxWidth()) {
            CozyText(
                when {
                    unpair -> "Desemparejar consola"
                    removing -> "Desinstalar agente"
                    else -> "Añadir consola"
                },
                size = 22,
                color = CozyInk,
                family = BalooXB
            )
            Spacer(Modifier.height(4.dp))
            CozyText(
                when {
                    unpair -> "Se borra el agente de la consola y se desempareja."
                    removing -> "Se parará el servicio y se borrará el binario."
                    else -> "Se instala por SSH y se empareja sola."
                },
                size = 14,
                color = CozyInkSoft
            )

            if (state.log.isNotEmpty()) {
                Spacer(Modifier.height(10.dp))
                Column(
                    Modifier
                        .fillMaxWidth()
                        .heightIn(max = 150.dp)
                        .clip(RoundedCornerShape(13.dp))
                        .background(CozyField)
                        .border(2.dp, CozyFieldBorder, RoundedCornerShape(13.dp))
                        .verticalScroll(rememberScrollState())
                        .padding(10.dp)
                ) {
                    state.log.takeLast(14).forEach { line ->
                        CozyText("> $line", size = 13, color = CozyInkSoft)
                    }
                }
            }

            state.error?.let {
                Spacer(Modifier.height(8.dp))
                CozyText("Error: $it", size = 14, color = CozyDanger)
            }

            when {
                state.installed -> {
                    Spacer(Modifier.height(12.dp))
                    CozyText("Consola instalada y emparejada.", size = 15, color = CozyPrimary)
                    Spacer(Modifier.height(10.dp))
                    CozyButton("Cerrar", onClose, style = CozyButtonStyle.Ghost)
                }

                state.uninstalled -> {
                    Spacer(Modifier.height(12.dp))
                    CozyText(
                        "Agente desinstalado de la consola.",
                        size = 15,
                        color = CozyPrimary
                    )
                    Spacer(Modifier.height(10.dp))
                    CozyButton("Cerrar", onClose, style = CozyButtonStyle.Ghost)
                }

                state.running -> {
                    Spacer(Modifier.height(12.dp))
                    CozyText("Trabajando… no cierres la app.", size = 15, color = CozyPrimary)
                }

                else -> {
                    Spacer(Modifier.height(10.dp))
                    CozyField(host, "IP de la consola") { host = it }
                    Spacer(Modifier.height(8.dp))
                    CozyField(user, "Usuario SSH") { user = it }
                    Spacer(Modifier.height(8.dp))
                    CozyField(pass, "Contraseña SSH", secret = true) { pass = it }
                    if (!removing) {
                        Spacer(Modifier.height(8.dp))
                        CozyText("El modelo se lee de la consola.", size = 13, color = CozyPlaceholder)
                    } else if (!unpair) {
                        Spacer(Modifier.height(8.dp))
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            CozySwitch(purge) { purge = it }
                            Spacer(Modifier.width(8.dp))
                            CozyText("Borrar datos del agente", size = 14, color = CozyInkSoft)
                        }
                    }
                    Spacer(Modifier.height(12.dp))
                    if (removing) {
                        CozyButton(
                            if (unpair) "Borrar agente y desemparejar" else "Desinstalar",
                            { onUninstall(host, user, pass, purge) },
                            style = CozyButtonStyle.Danger
                        )
                    } else {
                        CozyButton("Añadir e instalar", { onInstall(host, user, pass) })
                    }
                    if (unpair && state.error != null) {
                        Spacer(Modifier.height(10.dp))
                        CozyButton(
                            "Desemparejar sin borrar agente",
                            onUnpairOnly,
                            style = CozyButtonStyle.GhostDanger
                        )
                    }
                    Spacer(Modifier.height(10.dp))
                    CozyButton("Cancelar", onClose, style = CozyButtonStyle.Ghost)
                }
            }
        }
    }
}

@Composable
private fun CozySwitch(checked: Boolean, onChange: (Boolean) -> Unit) {
    val shape = RoundedCornerShape(999.dp)
    Box(
        Modifier
            .size(width = 48.dp, height = 28.dp)
            .clip(shape)
            .background(if (checked) CozySecondary else CozyFieldBorder)
            .clickable { onChange(!checked) }
            .padding(3.dp),
        contentAlignment = if (checked) Alignment.CenterEnd else Alignment.CenterStart
    ) {
        Box(
            Modifier
                .size(22.dp)
                .clip(RoundedCornerShape(999.dp))
                .background(CozyCard)
                .border(1.dp, CozyCardBorder, RoundedCornerShape(999.dp))
        )
    }
}

@Composable
private fun ManualDialog(onDismiss: () -> Unit, onAdd: (String, String) -> Unit) {
    var host by remember { mutableStateOf("192.168.1.") }
    var port by remember { mutableStateOf("47820") }
    Dialog(onDismissRequest = onDismiss) {
        CozyCard(Modifier.fillMaxWidth()) {
            CozyText("Añadir por IP", size = 22, color = CozyInk, family = BalooXB)
            Spacer(Modifier.height(12.dp))
            CozyField(host, "IP de la consola") { host = it }
            Spacer(Modifier.height(8.dp))
            CozyField(port, "Puerto", numeric = true) { port = it }
            Spacer(Modifier.height(14.dp))
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                CozyButton("Añadir", { onAdd(host, port) })
                CozyButton("Cancelar", onDismiss, style = CozyButtonStyle.Ghost)
            }
        }
    }
}

// ---------------------------------------------------------------- helpers ---

private fun slotBadge(a: Artifact): String = when {
    a.kind == "sram" -> "S"
    a.kind == "rtc" -> "R"
    a.slot == "auto" -> "A"
    a.slot.isEmpty() -> "0"
    else -> a.slot
}

private fun artifactLabel(a: Artifact): String = when {
    a.kind == "sram" -> "SRAM · batería"
    a.kind == "rtc" -> "RTC · reloj"
    a.slot == "auto" -> "Auto · al salir"
    a.slot == "0" -> "Slot 0"
    else -> "Slot ${a.slot}"
}

private fun dateLabel(epoch: Long): String {
    if (epoch <= 0) return "—"
    return SimpleDateFormat("dd MMM HH:mm", Locale.getDefault()).format(Date(epoch * 1000))
}

fun humanSize(n: Long): String {
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

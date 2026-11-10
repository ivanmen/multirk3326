package org.multirk.app.data

import org.json.JSONArray
import org.json.JSONObject
import java.io.BufferedInputStream
import java.io.BufferedOutputStream
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.io.Closeable
import java.io.DataInputStream
import java.io.DataOutputStream
import java.io.IOException
import java.io.OutputStream
import java.net.InetSocketAddress
import java.net.Socket
import java.security.MessageDigest
import java.util.zip.GZIPInputStream

object Op {
    const val HELLO = 1
    const val PAIR = 2
    const val INFO = 3
    const val SYSTEMS = 4
    const val GAMES = 5
    const val GAME = 6
    const val GET = 7
    const val CHUNK = 8
    const val END = 9
    const val PUT = 10
    const val READY = 11
    const val PUT_OK = 12
    const val MEDIA = 13
    const val REFRESH = 14
    const val PING = 15
    const val CONTROL = 16
    const val STATUS = 17
    const val CATALOG = 18
    const val SCRAPE = 19
    const val SETMETA = 20
    const val DELETE = 21
    const val AUDIT = 22
    const val LAUNCH = 23
    const val STOP = 24
    const val ERROR = 255
}

private data class Frame(val op: Int, val id: Long, val flags: Int, val payload: ByteArray)

private const val MAGIC = 0x4d4b
private const val VERSION = 2
private const val MAX_FRAME = 32 * 1024 * 1024
private const val CHUNK = 256 * 1024

class MultirkClient(
    private val host: String,
    private val port: Int = 47820,
    var token: String? = null
) : Closeable {

    private val socket = Socket()
    private var input: DataInputStream? = null
    private var output: DataOutputStream? = null
    private var nextId = 0L

    var authorized = false
        private set

    var info: JSONObject = JSONObject()
        private set

    fun connect(timeoutMs: Int = 4000) {
        if (socket.isConnected) return
        socket.tcpNoDelay = true
        socket.connect(InetSocketAddress(host, port), timeoutMs)
        socket.soTimeout = 30_000
        input = DataInputStream(BufferedInputStream(socket.getInputStream(), 1 shl 16))
        output = DataOutputStream(BufferedOutputStream(socket.getOutputStream(), 1 shl 16))
        val hello = call(
            Op.HELLO,
            JSONObject()
                .put("token", token ?: "")
                .put("client", "Android")
                .put("proto", VERSION)
        )
        info = hello
        authorized = hello.optBoolean("ok", false)
    }

    fun pair(clientName: String): String {
        val r = call(Op.PAIR, JSONObject().put("client", clientName))
        val t = r.optString("token")
        if (t.isEmpty()) throw IOException("la consola no devolvió token")
        token = t
        authorized = true
        return t
    }

    fun systems(): List<SystemInfo> {
        val r = call(Op.SYSTEMS, null)
        val arr = r.optJSONArray("systems") ?: JSONArray()
        return (0 until arr.length()).map { i -> parseSystem(arr.getJSONObject(i)) }
    }

    fun infoFull(): JSONObject = call(Op.INFO, null)

    /**
     * Whole catalogue in one gzip frame (the server caches the blob in RAM).
     * Pass the last stamp; the console answers notModified when nothing changed.
     */
    fun catalog(since: String?, system: String? = null): CatalogData {
        val req = JSONObject()
        if (!since.isNullOrBlank()) req.put("since", since)
        if (!system.isNullOrBlank()) req.put("system", system)
        val r = call(Op.CATALOG, req)
        if (r.optBoolean("notModified", false)) {
            return CatalogData(
                notModified = true,
                stamp = r.optString("stamp", since ?: ""),
                indexedAt = r.optString("indexedAt"),
                systems = emptyList(),
                games = emptyList()
            )
        }
        val systemsArr = r.optJSONArray("systems") ?: JSONArray()
        val gamesArr = r.optJSONArray("games") ?: JSONArray()
        val systems = (0 until systemsArr.length()).map { parseSystem(systemsArr.getJSONObject(it)) }
        val games = (0 until gamesArr.length()).map { parseGame(gamesArr.getJSONObject(it)) }
        return CatalogData(
            notModified = false,
            stamp = r.optString("stamp"),
            indexedAt = r.optString("indexedAt"),
            systems = systems,
            games = games
        )
    }

    private fun parseSystem(o: JSONObject) = SystemInfo(
        key = o.optString("key"),
        name = o.optString("name"),
        games = o.optInt("games"),
        withState = o.optInt("withState")
    )

    fun games(
        system: String? = null,
        query: String? = null,
        matchKey: String? = null,
        onlyState: Boolean = false,
        offset: Int = 0,
        limit: Int = 500
    ): Pair<Int, List<Game>> {
        val req = JSONObject()
            .put("offset", offset)
            .put("limit", limit)
            .put("hasState", onlyState)
        if (!system.isNullOrBlank()) req.put("system", system)
        if (!query.isNullOrBlank()) req.put("q", query)
        if (!matchKey.isNullOrBlank()) req.put("matchKey", matchKey)
        val r = call(Op.GAMES, req)
        val arr = r.optJSONArray("games") ?: JSONArray()
        val list = (0 until arr.length()).map { parseGame(arr.getJSONObject(it)) }
        return r.optInt("total", list.size) to list
    }

    fun game(id: String): Pair<Game?, List<Artifact>> {
        val r = call(Op.GAME, JSONObject().put("id", id))
        val g = r.optJSONObject("game")?.let { parseGame(it) }
        val arr = r.optJSONArray("artifacts") ?: JSONArray()
        val arts = (0 until arr.length()).map { i ->
            val o = arr.getJSONObject(i)
            Artifact(
                kind = o.optString("kind"),
                slot = o.optString("slot"),
                file = o.optString("file"),
                size = o.optLong("size"),
                mtime = o.optLong("mtime"),
                thumb = o.optBoolean("thumb")
            )
        }
        return g to arts
    }

    /** The console downloads data + cover, patches gamelist.xml and re-indexes. */
    fun scrape(
        gameId: String, lang: String, user: String, pass: String, devId: String, devPass: String,
        keepImage: Boolean = false
    ): JSONObject {
        socket.soTimeout = 120_000
        val ss = JSONObject().put("user", user).put("pass", pass)
            .put("devid", devId).put("devpass", devPass)
        return call(
            Op.SCRAPE,
            JSONObject().put("id", gameId).put("lang", lang).put("ss", ss).put("keepImage", keepImage)
        )
    }

    /** Makes the console re-read its SD: one system, or all of them when null. */
    fun refresh(system: String?) {
        socket.soTimeout = 120_000
        val req = JSONObject().put("wait", true)
        if (!system.isNullOrBlank()) req.put("system", system)
        call(Op.REFRESH, req)
    }

    /** Compares every gamelist.xml with the ROMs and drops entries whose file is gone. */
    fun audit(dry: Boolean = false): JSONObject {
        socket.soTimeout = 300_000
        return call(Op.AUDIT, JSONObject().put("dry", dry))
    }

    /** Starts the game on the console (ES is closed meanwhile). */
    fun launchGame(gameId: String): JSONObject {
        socket.soTimeout = 30_000
        return call(Op.LAUNCH, JSONObject().put("id", gameId))
    }

    /** Ends the running game; the console brings EmulationStation back. */
    fun stopGame(): JSONObject {
        socket.soTimeout = 30_000
        return call(Op.STOP, JSONObject())
    }

    /** Deletes the ROM and its media from the console's SD (irreversible). */
    fun deleteGame(gameId: String): JSONObject {
        socket.soTimeout = 60_000
        return call(Op.DELETE, JSONObject().put("id", gameId))
    }

    /** Manual info for systems ScreenScraper does not cover. */
    fun setMeta(gameId: String, meta: JSONObject): JSONObject {
        socket.soTimeout = 60_000
        return call(Op.SETMETA, JSONObject().put("id", gameId).put("meta", meta))
    }

    fun status(): CtlState {
        val o = call(Op.STATUS, null)
        return parseCtl(o)
    }

    fun setVolume(value: Int): CtlState =
        parseCtl(call(Op.CONTROL, JSONObject().put("action", "volume").put("value", value)))

    fun setBrightness(value: Int): CtlState =
        parseCtl(call(Op.CONTROL, JSONObject().put("action", "brightness").put("value", value)))

    fun setMute(mute: Boolean): CtlState =
        parseCtl(call(Op.CONTROL, JSONObject().put("action", "mute").put("mute", mute)))

    fun refresh(wait: Boolean) {
        call(Op.REFRESH, JSONObject().put("wait", wait))
    }

    fun download(gameId: String, kind: String, slot: String, out: OutputStream): FileMeta {
        nextId += 1
        val req = JSONObject().put("id", gameId).put("kind", kind).put("slot", slot)
        writeFrame(Op.GET, nextId, 0, req.toString().toByteArray(Charsets.UTF_8))
        flush()
        val metaFrame = readFrame()
        if (metaFrame.op == Op.ERROR) throw errorFrom(metaFrame)
        val meta = JSONObject(String(metaFrame.payload, Charsets.UTF_8))
        while (true) {
            val f = readFrame()
            when (f.op) {
                Op.CHUNK -> out.write(f.payload)
                Op.END -> return FileMeta(meta.optLong("size"), meta.optLong("mtime"), meta.optString("name"))
                Op.ERROR -> throw errorFrom(f)
            }
        }
    }

    fun upload(
        gameId: String,
        kind: String,
        slot: String,
        data: ByteArray,
        mtime: Long,
        force: Boolean = false,
        ifMatch: String? = null
    ): JSONObject {
        val sha = sha256(data)
        val req = JSONObject()
            .put("id", gameId)
            .put("kind", kind)
            .put("slot", slot)
            .put("size", data.size)
            .put("sha256", sha)
            .put("mtime", mtime)
            .put("force", force)
        if (!ifMatch.isNullOrBlank()) req.put("ifMatch", ifMatch)

        nextId += 1
        val id = nextId
        writeFrame(Op.PUT, id, 0, req.toString().toByteArray(Charsets.UTF_8))
        flush()
        val ready = readFrame()
        if (ready.op == Op.ERROR) throw errorFrom(ready)

        var off = 0
        while (off < data.size) {
            val n = minOf(CHUNK, data.size - off)
            writeFrame(Op.CHUNK, id, 0, data.copyOfRange(off, off + n))
            off += n
        }
        writeFrame(Op.END, id, 0, ByteArray(0))
        flush()

        val resp = readFrame()
        if (resp.op == Op.ERROR) throw errorFrom(resp)
        return JSONObject(String(resp.payload, Charsets.UTF_8))
    }

    fun media(gameId: String, type: String): ByteArray {
        nextId += 1
        val req = JSONObject().put("id", gameId).put("type", type)
        writeFrame(Op.MEDIA, nextId, 0, req.toString().toByteArray(Charsets.UTF_8))
        flush()
        val meta = readFrame()
        if (meta.op == Op.ERROR) throw errorFrom(meta)
        val out = ByteArrayOutputStream()
        while (true) {
            val f = readFrame()
            when (f.op) {
                Op.CHUNK -> out.write(f.payload)
                Op.END -> return out.toByteArray()
                Op.ERROR -> throw errorFrom(f)
            }
        }
    }

    private fun call(op: Int, req: JSONObject?): JSONObject {
        nextId += 1
        val bytes = req?.toString()?.toByteArray(Charsets.UTF_8) ?: ByteArray(0)
        writeFrame(op, nextId, 0, bytes)
        flush()
        val f = readFrame()
        if (f.op == Op.ERROR) throw errorFrom(f)
        return JSONObject(String(f.payload, Charsets.UTF_8))
    }

    private fun writeFrame(op: Int, id: Long, flags: Int, payload: ByteArray) {
        val out = output ?: throw IOException("socket cerrado")
        out.writeShort(MAGIC)
        out.writeByte(VERSION)
        out.writeByte(op)
        out.writeInt(id.toInt())
        out.writeInt(flags)
        out.writeInt(payload.size)
        if (payload.isNotEmpty()) out.write(payload)
    }

    private fun flush() {
        output?.flush()
    }

    private fun readFrame(): Frame {
        val inp = input ?: throw IOException("socket cerrado")
        val magic = inp.readUnsignedShort()
        if (magic != MAGIC) throw IOException("respuesta no es MULTIRK/2")
        val version = inp.readUnsignedByte()
        if (version != VERSION) throw IOException("protocolo $version no soportado")
        val op = inp.readUnsignedByte()
        val id = inp.readInt().toLong() and 0xffffffffL
        val flags = inp.readInt()
        val len = inp.readInt()
        if (len < 0 || len > MAX_FRAME) throw IOException("frame inválido ($len)")
        val raw = ByteArray(len)
        inp.readFully(raw)
        val payload = if (flags and 1 != 0) gunzip(raw) else raw
        return Frame(op, id, flags, payload)
    }

    private fun errorFrom(f: Frame): Exception {
        val obj = runCatching { JSONObject(String(f.payload, Charsets.UTF_8)) }.getOrNull()
        if (obj?.optInt("status", 0) == 401) return AuthRequiredException()
        return ConsoleException(obj?.optString("error") ?: "error de la consola")
    }

    private fun gunzip(raw: ByteArray): ByteArray =
        GZIPInputStream(ByteArrayInputStream(raw)).use { it.readBytes() }

    private fun sha256(data: ByteArray): String =
        MessageDigest.getInstance("SHA-256").digest(data)
            .joinToString("") { "%02x".format(it) }

    private fun parseGame(o: JSONObject) = Game(
        id = o.optString("id"),
        system = o.optString("system"),
        name = o.optString("name"),
        file = o.optString("file"),
        size = o.optLong("size"),
        mtime = o.optLong("mtime"),
        hasState = o.optBoolean("hasState"),
        stateMtime = o.optLong("stateMtime"),
        hasSram = o.optBoolean("hasSram"),
        hasImage = o.optBoolean("hasImage"),
        missing = o.optBoolean("missing"),
        matchKey = o.optString("matchKey"),
        desc = o.optString("desc"),
        developer = o.optString("developer"),
        publisher = o.optString("publisher"),
        genre = o.optString("genre"),
        players = o.optString("players"),
        releasedate = o.optString("releasedate"),
        rating = o.optDouble("rating", 0.0),
        scrapable = o.optBoolean("scrapable", true)
    )

    private fun parseCtl(o: JSONObject) = CtlState(
        volume = if (o.has("volume")) o.optInt("volume") else null,
        muted = if (o.has("muted")) o.optBoolean("muted") else null,
        brightness = if (o.has("brightness")) o.optInt("brightness") else null,
        battery = if (o.has("battery")) o.optInt("battery") else null,
        charging = if (o.has("charging")) o.optBoolean("charging") else null,
        audio = if (o.has("audio")) o.optString("audio") else null,
        backlight = if (o.has("backlightDriver")) o.optString("backlightDriver") else null,
        error = if (o.has("error")) o.optString("error") else null
    )

    override fun close() {
        runCatching { socket.close() }
    }
}

fun parseDeviceInfo(o: JSONObject): DeviceInfo {
    val s = o.optJSONObject("system") ?: JSONObject()
    return DeviceInfo(
        id = o.optString("id"),
        name = o.optString("name"),
        version = o.optString("version"),
        model = o.optString("model"),
        os = o.optString("os"),
        romsRoot = o.optString("romsRoot"),
        systems = o.optInt("systems"),
        games = o.optInt("games"),
        withState = o.optInt("withState"),
        indexedAt = o.optString("indexedAt"),
        statesAt = o.optString("statesAt"),
        protocol = o.optString("protocol"),
        sys = SysInfo(
            cpu = s.optString("cpu"),
            cores = s.optInt("cores"),
            load1 = s.optDouble("load1", 0.0),
            memTotalMB = s.optLong("memTotalMB"),
            memAvailMB = s.optLong("memAvailMB"),
            diskTotalGB = s.optDouble("diskTotalGB", 0.0),
            diskFreeGB = s.optDouble("diskFreeGB", 0.0),
            tempC = s.optDouble("tempC", 0.0),
            kernel = s.optString("kernel"),
            uptimeSec = s.optLong("uptimeSec")
        )
    )
}

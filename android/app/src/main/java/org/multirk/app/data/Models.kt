package org.multirk.app.data

data class Device(
    val id: String,
    val name: String,
    val host: String,
    val port: Int = 47820,
    val token: String? = null,
    val paired: Boolean = false,
    val model: String = "",
    val systems: Int = 0,
    val games: Int = 0,
    val lastSeen: Long = 0
)

data class SystemInfo(
    val key: String,
    val name: String,
    val games: Int,
    val withState: Int
)

data class Game(
    val id: String,
    val system: String,
    val name: String,
    val file: String,
    val size: Long,
    val mtime: Long,
    val hasState: Boolean,
    val stateMtime: Long,
    val hasSram: Boolean,
    val hasImage: Boolean,
    val missing: Boolean,
    val matchKey: String,
    val desc: String = "",
    val developer: String = "",
    val publisher: String = "",
    val genre: String = "",
    val players: String = "",
    val releasedate: String = "",
    val rating: Double = 0.0,
    val scrapable: Boolean = true
)

data class Artifact(
    val kind: String,
    val slot: String,
    val file: String,
    val size: Long,
    val mtime: Long,
    val thumb: Boolean
)

data class FileMeta(
    val size: Long,
    val mtime: Long,
    val name: String
)

data class CatalogData(
    val notModified: Boolean,
    val stamp: String,
    val indexedAt: String,
    val systems: List<SystemInfo>,
    val games: List<Game>
)

data class SysInfo(
    val cpu: String = "",
    val cores: Int = 0,
    val load1: Double = 0.0,
    val memTotalMB: Long = 0,
    val memAvailMB: Long = 0,
    val diskTotalGB: Double = 0.0,
    val diskFreeGB: Double = 0.0,
    val tempC: Double = 0.0,
    val kernel: String = "",
    val uptimeSec: Long = 0
)

data class DeviceInfo(
    val id: String,
    val name: String,
    val version: String,
    val model: String,
    val os: String,
    val romsRoot: String,
    val systems: Int,
    val games: Int,
    val withState: Int,
    val indexedAt: String,
    val statesAt: String,
    val protocol: String,
    val sys: SysInfo
)

data class CtlState(
    val volume: Int? = null,
    val muted: Boolean? = null,
    val brightness: Int? = null,
    val battery: Int? = null,
    val charging: Boolean? = null,
    val audio: String? = null,
    val backlight: String? = null,
    val error: String? = null
)

data class SyncEvent(
    val ts: Long,
    val game: String,
    val from: String,
    val to: String,
    val kind: String,
    val slot: String,
    val bytes: Long,
    val ok: Boolean,
    val detail: String
)

class AuthRequiredException : Exception("La consola necesita emparejamiento")

/** The console answered, but with an error (not a connection problem). */
class ConsoleException(message: String) : java.io.IOException(message)

/** True when the console could not be reached or the link dropped. */
fun Throwable.isConnectionLoss(): Boolean =
    this is java.io.IOException && this !is ConsoleException

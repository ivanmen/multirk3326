package org.multirk.app.data

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper

/**
 * Systems hidden until the user decides otherwise: folders that are not game
 * systems. The user can hide more (long press on a system) or show them again
 * (Ajustes → Sistemas ocultos); the choice is stored per console.
 */
private val DEFAULT_HIDDEN = listOf(
    "tools", "easyrpg", "found.000", "launchimages", "themes",
    "system volume information", "indexervolumeguid", "wpsettings"
)

/** ES launcher entries ("Scan for new games"), not real games: never shown. */
private const val SCAN_SQL =
    "LOWER(REPLACE(file, '_', ' ')) LIKE 'scan for new games%' OR LOWER(REPLACE(name, '_', ' ')) LIKE 'scan for new games%'"

class Store(context: Context) : SQLiteOpenHelper(context, "multirk.db", null, 3) {

    override fun onCreate(db: SQLiteDatabase) {
        db.execSQL(
            """CREATE TABLE devices(
                id TEXT PRIMARY KEY, name TEXT, host TEXT, port INTEGER,
                token TEXT, paired INTEGER, model TEXT, systems INTEGER,
                games INTEGER, lastSeen INTEGER)"""
        )
        db.execSQL(
            """CREATE TABLE systems(
                deviceId TEXT, key TEXT, name TEXT, games INTEGER, withState INTEGER,
                PRIMARY KEY(deviceId, key))"""
        )
        db.execSQL(
            """CREATE TABLE games(
                deviceId TEXT, id TEXT, system TEXT, name TEXT, file TEXT,
                size INTEGER, mtime INTEGER, hasState INTEGER, stateMtime INTEGER,
                hasSram INTEGER, hasImage INTEGER, missing INTEGER, matchKey TEXT,
                descr TEXT, developer TEXT, publisher TEXT, genre TEXT, players TEXT,
                releasedate TEXT, rating REAL, scrapable INTEGER DEFAULT 1,
                PRIMARY KEY(deviceId, id))"""
        )
        db.execSQL(
            """CREATE TABLE artifacts(
                deviceId TEXT, gameId TEXT, kind TEXT, slot TEXT, file TEXT,
                size INTEGER, mtime INTEGER, thumb INTEGER,
                PRIMARY KEY(deviceId, gameId, kind, slot))"""
        )
        db.execSQL(
            """CREATE TABLE sync_log(
                id INTEGER PRIMARY KEY AUTOINCREMENT, ts INTEGER, game TEXT,
                fromDevice TEXT, toDevice TEXT, kind TEXT, slot TEXT,
                bytes INTEGER, ok INTEGER, detail TEXT)"""
        )
        db.execSQL("CREATE INDEX idx_games_device ON games(deviceId, name)")
        db.execSQL("CREATE INDEX idx_games_state ON games(deviceId, hasState)")
        db.execSQL(
            "CREATE TABLE IF NOT EXISTS meta(deviceId TEXT, key TEXT, value TEXT, PRIMARY KEY(deviceId, key))"
        )
    }

    override fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int) {
        if (oldVersion < 3) {
            for (col in listOf(
                "descr TEXT", "developer TEXT", "publisher TEXT", "genre TEXT",
                "players TEXT", "releasedate TEXT", "rating REAL", "scrapable INTEGER DEFAULT 1"
            )) {
                db.execSQL("ALTER TABLE games ADD COLUMN $col")
            }
            // The cached catalogue carries no gamelist data yet: drop it so the next
            // open downloads the compressed catalogue again, now with everything.
            db.execSQL("DELETE FROM games")
            db.execSQL("DELETE FROM systems")
            db.execSQL("DELETE FROM meta WHERE key = 'catalogStamp'")
        }
        if (oldVersion < 2) {
            db.execSQL(
                "CREATE TABLE IF NOT EXISTS meta(deviceId TEXT, key TEXT, value TEXT, PRIMARY KEY(deviceId, key))"
            )
        }
    }

    fun setMeta(deviceId: String, key: String, value: String) {
        val cv = ContentValues().apply {
            put("deviceId", deviceId)
            put("key", key)
            put("value", value)
        }
        writableDatabase.insertWithOnConflict("meta", null, cv, SQLiteDatabase.CONFLICT_REPLACE)
    }

    fun meta(deviceId: String, key: String): String? {
        readableDatabase.query(
            "meta", arrayOf("value"), "deviceId = ? AND key = ?", arrayOf(deviceId, key),
            null, null, null
        ).use { c ->
            return if (c.moveToFirst()) c.getString(0) else null
        }
    }

    fun hasCatalog(deviceId: String): Boolean {
        readableDatabase.rawQuery(
            "SELECT 1 FROM games WHERE deviceId = ? LIMIT 1", arrayOf(deviceId)
        ).use { c ->
            return c.moveToFirst()
        }
    }

    fun upsertDevice(d: Device) {
        val cv = ContentValues().apply {
            put("id", d.id)
            put("name", d.name)
            put("host", d.host)
            put("port", d.port)
            put("token", d.token)
            put("paired", if (d.paired) 1 else 0)
            put("model", d.model)
            put("systems", d.systems)
            put("games", d.games)
            put("lastSeen", d.lastSeen)
        }
        writableDatabase.insertWithOnConflict("devices", null, cv, SQLiteDatabase.CONFLICT_REPLACE)
    }

    fun devices(): List<Device> {
        val out = ArrayList<Device>()
        readableDatabase.query("devices", null, null, null, null, null, "name COLLATE NOCASE").use { c ->
            while (c.moveToNext()) {
                out.add(
                    Device(
                        id = c.getString(c.getColumnIndexOrThrow("id")),
                        name = c.getString(c.getColumnIndexOrThrow("name")),
                        host = c.getString(c.getColumnIndexOrThrow("host")),
                        port = c.getInt(c.getColumnIndexOrThrow("port")),
                        token = c.getString(c.getColumnIndexOrThrow("token")),
                        paired = c.getInt(c.getColumnIndexOrThrow("paired")) == 1,
                        model = c.getString(c.getColumnIndexOrThrow("model")) ?: "",
                        systems = c.getInt(c.getColumnIndexOrThrow("systems")),
                        games = c.getInt(c.getColumnIndexOrThrow("games")),
                        lastSeen = c.getLong(c.getColumnIndexOrThrow("lastSeen"))
                    )
                )
            }
        }
        return out
    }

    fun device(id: String): Device? = devices().firstOrNull { it.id == id }

    fun deleteDevice(id: String) {
        writableDatabase.delete("devices", "id = ?", arrayOf(id))
    }

    fun saveSystems(deviceId: String, list: List<SystemInfo>) {
        val db = writableDatabase
        db.beginTransaction()
        try {
            db.delete("systems", "deviceId = ?", arrayOf(deviceId))
            for (s in list) {
                val cv = ContentValues().apply {
                    put("deviceId", deviceId)
                    put("key", s.key)
                    put("name", s.name)
                    put("games", s.games)
                    put("withState", s.withState)
                }
                db.insertWithOnConflict("systems", null, cv, SQLiteDatabase.CONFLICT_REPLACE)
            }
            db.setTransactionSuccessful()
        } finally {
            db.endTransaction()
        }
    }

    private val hiddenCache = HashMap<String, Set<String>>()

    /** Lower-case keys of the systems hidden for this console. */
    @Synchronized
    fun hiddenSystems(deviceId: String): Set<String> = hiddenCache.getOrPut(deviceId) {
        meta(deviceId, "hiddenSystems")
            ?.split('\n')?.filter { it.isNotBlank() }?.toSet()
            ?: DEFAULT_HIDDEN.toSet()
    }

    @Synchronized
    fun setHidden(deviceId: String, key: String, hidden: Boolean) {
        val set = hiddenSystems(deviceId).toMutableSet()
        if (hidden) set.add(key.lowercase()) else set.remove(key.lowercase())
        setMeta(deviceId, "hiddenSystems", set.sorted().joinToString("\n"))
        hiddenCache[deviceId] = set
    }

    /** SQL fragment (+ args) that leaves out the hidden systems of this console. */
    private fun notHidden(deviceId: String, col: String): Pair<String, List<String>> {
        val h = hiddenSystems(deviceId).toList()
        if (h.isEmpty()) return "1 = 1" to emptyList()
        return "LOWER($col) NOT IN (${h.joinToString(",") { "?" }})" to h
    }

    /** The hidden systems that exist on the console, to show them in Ajustes. */
    fun hiddenSystemInfos(deviceId: String): List<SystemInfo> {
        val h = hiddenSystems(deviceId)
        val out = ArrayList<SystemInfo>()
        readableDatabase.query(
            "systems", null, "deviceId = ?", arrayOf(deviceId), null, null, "name COLLATE NOCASE"
        ).use { c ->
            while (c.moveToNext()) {
                val key = c.getString(c.getColumnIndexOrThrow("key"))
                if (key.lowercase() in h) {
                    out.add(
                        SystemInfo(
                            key = key,
                            name = c.getString(c.getColumnIndexOrThrow("name")),
                            games = c.getInt(c.getColumnIndexOrThrow("games")),
                            withState = c.getInt(c.getColumnIndexOrThrow("withState"))
                        )
                    )
                }
            }
        }
        return out
    }

    fun systems(deviceId: String): List<SystemInfo> {
        val out = ArrayList<SystemInfo>()
        val (hidSql, hidArgs) = notHidden(deviceId, "s.key")
        // The game count is what is actually shown (no hidden launcher entries,
        // no missing files); a system left with no games is not listed.
        readableDatabase.rawQuery(
            "SELECT s.key, s.name, s.withState, " +
                "(SELECT COUNT(*) FROM games g WHERE g.deviceId = s.deviceId " +
                "AND g.system = s.key AND g.missing = 0 AND NOT ($SCAN_SQL)) AS n " +
                "FROM systems s WHERE s.deviceId = ? AND $hidSql " +
                "AND LOWER(s.key) NOT LIKE '%backup%' " +
                "ORDER BY s.name COLLATE NOCASE",
            (listOf(deviceId) + hidArgs).toTypedArray()
        ).use { c ->
            while (c.moveToNext()) {
                val n = c.getInt(3)
                if (n <= 0) continue
                out.add(
                    SystemInfo(
                        key = c.getString(0),
                        name = c.getString(1),
                        games = n,
                        withState = c.getInt(2)
                    )
                )
            }
        }
        return out
    }

    private fun insertGames(db: SQLiteDatabase, deviceId: String, list: List<Game>) {
        for (g in list) {
                val cv = ContentValues().apply {
                    put("deviceId", deviceId)
                    put("id", g.id)
                    put("system", g.system)
                    put("name", g.name)
                    put("file", g.file)
                    put("size", g.size)
                    put("mtime", g.mtime)
                    put("hasState", if (g.hasState) 1 else 0)
                    put("stateMtime", g.stateMtime)
                    put("hasSram", if (g.hasSram) 1 else 0)
                    put("hasImage", if (g.hasImage) 1 else 0)
                    put("missing", if (g.missing) 1 else 0)
                    put("matchKey", g.matchKey)
                    put("descr", g.desc)
                    put("developer", g.developer)
                    put("publisher", g.publisher)
                    put("genre", g.genre)
                    put("players", g.players)
                    put("releasedate", g.releasedate)
                    put("rating", g.rating)
                    put("scrapable", if (g.scrapable) 1 else 0)
                }
                db.insertWithOnConflict("games", null, cv, SQLiteDatabase.CONFLICT_REPLACE)
        }
    }

    /** Replaces one system (its row and its games) and leaves the rest of the cache alone. */
    fun replaceSystem(deviceId: String, key: String, systems: List<SystemInfo>, games: List<Game>) {
        val db = writableDatabase
        db.beginTransaction()
        try {
            db.delete("games", "deviceId = ? AND system = ?", arrayOf(deviceId, key))
            db.delete("systems", "deviceId = ? AND key = ?", arrayOf(deviceId, key))
            for (s in systems) {
                val cv = ContentValues().apply {
                    put("deviceId", deviceId)
                    put("key", s.key)
                    put("name", s.name)
                    put("games", s.games)
                    put("withState", s.withState)
                }
                db.insertWithOnConflict("systems", null, cv, SQLiteDatabase.CONFLICT_REPLACE)
            }
            insertGames(db, deviceId, games)
            db.setTransactionSuccessful()
        } finally {
            db.endTransaction()
        }
    }

    fun saveGames(deviceId: String, list: List<Game>) {
        val db = writableDatabase
        db.beginTransaction()
        try {
            db.delete("games", "deviceId = ?", arrayOf(deviceId))
            insertGames(db, deviceId, list)
            db.setTransactionSuccessful()
        } finally {
            db.endTransaction()
        }
    }

    private fun gamesWhere(
        deviceId: String,
        system: String?,
        query: String?,
        onlyState: Boolean
    ): Pair<String, Array<String>> {
        val (hidSql, hidArgs) = notHidden(deviceId, "system")
        val where = StringBuilder(
            "deviceId = ? AND missing = 0 AND $hidSql " +
                "AND LOWER(system) NOT LIKE '%backup%' AND NOT ($SCAN_SQL)"
        )
        val args = arrayListOf(deviceId)
        args.addAll(hidArgs)
        if (!system.isNullOrBlank()) {
            where.append(" AND system = ?")
            args.add(system)
        }
        if (onlyState) where.append(" AND hasState = 1")
        if (!query.isNullOrBlank()) {
            where.append(" AND (name LIKE ? OR file LIKE ?)")
            args.add("%$query%")
            args.add("%$query%")
        }
        return where.toString() to args.toTypedArray()
    }

    fun countGames(
        deviceId: String,
        system: String? = null,
        query: String? = null,
        onlyState: Boolean = false
    ): Int {
        val (where, args) = gamesWhere(deviceId, system, query, onlyState)
        readableDatabase.rawQuery(
            "SELECT COUNT(*) FROM games WHERE $where", args
        ).use { c ->
            return if (c.moveToFirst()) c.getInt(0) else 0
        }
    }

    /** Up to [limit] names for the search box: prefix matches first, then the rest. */
    fun suggest(deviceId: String, system: String?, query: String, limit: Int = 5): List<Game> {
        val q = query.trim()
        if (q.isEmpty()) return emptyList()
        val (where, args) = gamesWhere(deviceId, system, q, false)
        val out = ArrayList<Game>()
        readableDatabase.rawQuery(
            "SELECT id, system, name, file FROM games WHERE $where " +
                "ORDER BY CASE WHEN name LIKE ? THEN 0 ELSE 1 END, name COLLATE NOCASE ASC LIMIT $limit",
            args + "$q%"
        ).use { c ->
            while (c.moveToNext()) {
                out.add(
                    Game(
                        id = c.getString(0), system = c.getString(1), name = c.getString(2),
                        file = c.getString(3), size = 0, mtime = 0, hasState = false,
                        stateMtime = 0, hasSram = false, hasImage = false, missing = false,
                        matchKey = ""
                    )
                )
            }
        }
        return out
    }

    fun games(
        deviceId: String,
        system: String? = null,
        query: String? = null,
        onlyState: Boolean = false,
        limit: Int = 500,
        offset: Int = 0
    ): List<Game> {
        val (where, args) = gamesWhere(deviceId, system, query, onlyState)
        val limitClause = if (offset > 0) "$limit OFFSET $offset" else limit.toString()
        val out = ArrayList<Game>()
        readableDatabase.query(
            "games", null, where, args,
            null, null, "name COLLATE NOCASE ASC", limitClause
        ).use { c ->
            while (c.moveToNext()) {
                out.add(rowToGame(c))
            }
        }
        return out
    }

    private fun android.database.Cursor.str(col: String): String =
        getString(getColumnIndexOrThrow(col)) ?: ""

    private fun rowToGame(c: android.database.Cursor) = Game(
        id = c.getString(c.getColumnIndexOrThrow("id")),
        system = c.getString(c.getColumnIndexOrThrow("system")),
        name = c.getString(c.getColumnIndexOrThrow("name")),
        file = c.getString(c.getColumnIndexOrThrow("file")),
        size = c.getLong(c.getColumnIndexOrThrow("size")),
        mtime = c.getLong(c.getColumnIndexOrThrow("mtime")),
        hasState = c.getInt(c.getColumnIndexOrThrow("hasState")) == 1,
        stateMtime = c.getLong(c.getColumnIndexOrThrow("stateMtime")),
        hasSram = c.getInt(c.getColumnIndexOrThrow("hasSram")) == 1,
        hasImage = c.getInt(c.getColumnIndexOrThrow("hasImage")) == 1,
        missing = c.getInt(c.getColumnIndexOrThrow("missing")) == 1,
        matchKey = c.str("matchKey"),
        desc = c.str("descr"),
        developer = c.str("developer"),
        publisher = c.str("publisher"),
        genre = c.str("genre"),
        players = c.str("players"),
        releasedate = c.str("releasedate"),
        rating = c.getDouble(c.getColumnIndexOrThrow("rating")),
        scrapable = c.getInt(c.getColumnIndexOrThrow("scrapable")) != 0
    )

    /** One game with everything the catalogue carried, gamelist data included. */
    fun game(deviceId: String, id: String): Game? =
        readableDatabase.query(
            "games", null, "deviceId = ? AND id = ?", arrayOf(deviceId, id),
            null, null, null, "1"
        ).use { c -> if (c.moveToFirst()) rowToGame(c) else null }

    /** Drops one game from the local cache (row, saves, system counter). */
    fun removeGame(deviceId: String, game: Game) {
        val db = writableDatabase
        db.beginTransaction()
        try {
            db.delete("games", "deviceId = ? AND id = ?", arrayOf(deviceId, game.id))
            db.delete("artifacts", "deviceId = ? AND gameId = ?", arrayOf(deviceId, game.id))
            db.execSQL(
                "UPDATE systems SET games = MAX(games - 1, 0) WHERE deviceId = ? AND key = ?",
                arrayOf(deviceId, game.system)
            )
            db.setTransactionSuccessful()
        } finally {
            db.endTransaction()
        }
    }

    fun saveArtifacts(deviceId: String, gameId: String, list: List<Artifact>) {
        val db = writableDatabase
        db.beginTransaction()
        try {
            db.delete("artifacts", "deviceId = ? AND gameId = ?", arrayOf(deviceId, gameId))
            for (a in list) {
                val cv = ContentValues().apply {
                    put("deviceId", deviceId)
                    put("gameId", gameId)
                    put("kind", a.kind)
                    put("slot", a.slot)
                    put("file", a.file)
                    put("size", a.size)
                    put("mtime", a.mtime)
                    put("thumb", if (a.thumb) 1 else 0)
                }
                db.insertWithOnConflict("artifacts", null, cv, SQLiteDatabase.CONFLICT_REPLACE)
            }
            db.setTransactionSuccessful()
        } finally {
            db.endTransaction()
        }
    }

    fun artifacts(deviceId: String, gameId: String): List<Artifact> {
        val out = ArrayList<Artifact>()
        readableDatabase.query(
            "artifacts", null, "deviceId = ? AND gameId = ?", arrayOf(deviceId, gameId),
            null, null, "kind, slot"
        ).use { c ->
            while (c.moveToNext()) {
                out.add(
                    Artifact(
                        kind = c.getString(c.getColumnIndexOrThrow("kind")),
                        slot = c.getString(c.getColumnIndexOrThrow("slot")),
                        file = c.getString(c.getColumnIndexOrThrow("file")),
                        size = c.getLong(c.getColumnIndexOrThrow("size")),
                        mtime = c.getLong(c.getColumnIndexOrThrow("mtime")),
                        thumb = c.getInt(c.getColumnIndexOrThrow("thumb")) == 1
                    )
                )
            }
        }
        return out
    }

    fun log(e: SyncEvent) {
        val cv = ContentValues().apply {
            put("ts", e.ts)
            put("game", e.game)
            put("fromDevice", e.from)
            put("toDevice", e.to)
            put("kind", e.kind)
            put("slot", e.slot)
            put("bytes", e.bytes)
            put("ok", if (e.ok) 1 else 0)
            put("detail", e.detail)
        }
        writableDatabase.insert("sync_log", null, cv)
    }

    fun log(limit: Int = 50): List<SyncEvent> {
        val out = ArrayList<SyncEvent>()
        readableDatabase.query(
            "sync_log", null, null, null, null, null, "ts DESC", limit.toString()
        ).use { c ->
            while (c.moveToNext()) {
                out.add(
                    SyncEvent(
                        ts = c.getLong(c.getColumnIndexOrThrow("ts")),
                        game = c.getString(c.getColumnIndexOrThrow("game")),
                        from = c.getString(c.getColumnIndexOrThrow("fromDevice")),
                        to = c.getString(c.getColumnIndexOrThrow("toDevice")),
                        kind = c.getString(c.getColumnIndexOrThrow("kind")),
                        slot = c.getString(c.getColumnIndexOrThrow("slot")),
                        bytes = c.getLong(c.getColumnIndexOrThrow("bytes")),
                        ok = c.getInt(c.getColumnIndexOrThrow("ok")) == 1,
                        detail = c.getString(c.getColumnIndexOrThrow("detail")) ?: ""
                    )
                )
            }
        }
        return out
    }

    fun clearCatalog(deviceId: String) {
        val db = writableDatabase
        db.beginTransaction()
        try {
            db.delete("games", "deviceId = ?", arrayOf(deviceId))
            db.delete("systems", "deviceId = ?", arrayOf(deviceId))
            db.delete("artifacts", "deviceId = ?", arrayOf(deviceId))
            // the user's hidden systems are a preference, not cache: keep them
            db.delete("meta", "deviceId = ? AND key <> 'hiddenSystems'", arrayOf(deviceId))
            db.setTransactionSuccessful()
        } finally {
            db.endTransaction()
        }
    }
}

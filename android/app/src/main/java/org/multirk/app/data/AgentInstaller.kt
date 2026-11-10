package org.multirk.app.data

import android.content.res.AssetManager
import com.jcraft.jsch.ChannelExec
import com.jcraft.jsch.ChannelSftp
import com.jcraft.jsch.JSch
import com.jcraft.jsch.Session
import org.json.JSONObject
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.util.Properties

/**
 * Installs/uninstalls the arm64/armv7 agent on an ArkOS console over SSH.
 * Uses password auth; ArkOS ships ark/ark and passwordless sudo.
 */
class AgentInstaller(
    private val host: String,
    private val user: String,
    private val password: String,
    private val port: Int = 22
) {
    private var session: Session? = null

    data class Exec(val code: Int, val output: String)
    data class Result(val home: String, val mode: String)

    fun connect() {
        val jsch = JSch()
        val s = jsch.getSession(user, host, port)
        s.setPassword(password)
        val cfg = Properties()
        cfg["StrictHostKeyChecking"] = "no"
        cfg["PreferredAuthentications"] = "password,keyboard-interactive"
        cfg["PubkeyAuthentication"] = "no"
        cfg["kex"] =
            "curve25519-sha256,curve25519-sha256@libssh.org,ecdh-sha2-nistp256," +
                "ecdh-sha2-nistp384,ecdh-sha2-nistp521,diffie-hellman-group14-sha256," +
                "diffie-hellman-group14-sha1"
        cfg["server_host_key"] = "ssh-ed25519,ecdsa-sha2-nistp256,rsa-sha2-256,ssh-rsa"
        cfg["ciphers"] = "aes128-ctr,aes192-ctr,aes256-ctr,aes128-gcm@openssh.com,aes256-gcm@openssh.com"
        s.setConfig(cfg)
        s.connect(15_000)
        session = s
    }

    fun close() {
        runCatching { session?.disconnect() }
        session = null
    }

    fun exec(command: String, timeoutMs: Int = 120_000): Exec {
        val s = session ?: error("sin sesión SSH")
        val ch = s.openChannel("exec") as ChannelExec
        val out = ByteArrayOutputStream()
        val err = ByteArrayOutputStream()
        ch.setCommand(command)
        ch.setOutputStream(out)
        ch.setErrStream(err)
        ch.connect(15_000)
        val deadline = System.currentTimeMillis() + timeoutMs
        while (!ch.isClosed && System.currentTimeMillis() < deadline) {
            Thread.sleep(40)
        }
        val code = ch.exitStatus
        ch.disconnect()
        return Exec(code, (out.toString("UTF-8") + err.toString("UTF-8")).trim())
    }

    private fun upload(bytes: ByteArray, remote: String, mode: Int) {
        val s = session ?: error("sin sesión SSH")
        val ch = s.openChannel("sftp") as ChannelSftp
        ch.connect(15_000)
        ByteArrayInputStream(bytes).use { ch.put(it, remote) }
        runCatching { ch.chmod(mode, remote) }
        ch.disconnect()
    }

    private fun sudo(commands: String): Exec {
        val direct = exec("sudo -n sh -c " + shq(commands))
        if (direct.code == 0) return direct
        return exec(
            "printf '%s\\n' " + shq(password) + " | sudo -S -p '' sh -c " + shq(commands)
        )
    }

    fun install(
        assets: AssetManager,
        consoleName: String,
        onLog: (String) -> Unit
    ): Result {
        connect()
        try {
            onLog("conectado a $host")
            val hw = exec(
                "grep -m1 -E '^(Hardware|Model)' /proc/cpuinfo | cut -d: -f2- | sed 's/^ *//'"
            ).output.lines().firstOrNull()?.trim().orEmpty()
            if (hw.isNotBlank()) onLog("modelo: $hw")
            val deviceName = hw.ifBlank { consoleName.ifBlank { host } }
            val arch = exec("uname -m").output.trim()
            onLog("arquitectura: $arch")
            val asset = when {
                arch.contains("aarch64") || arch.contains("arm64") -> "multirkd-linux-arm64"
                arch.startsWith("armv7") || arch.startsWith("armv8l") -> "multirkd-linux-armv7"
                else -> error("arquitectura no soportada: $arch")
            }
            val bytes = assets.open(asset).readBytes()
            onLog("subiendo ${asset} (${bytes.size / 1024 / 1024} MB)…")

            val home = exec("echo \$HOME").output.trim().ifBlank { "/home/$user" }
            val dataDir = "$home/.config/multirk"
            exec("mkdir -p '$home/multirk' '$dataDir'")
            upload(bytes, "$home/multirk/multirkd.new", 493)
            val mv = exec(
                "chmod +x '$home/multirk/multirkd.new' && " +
                    "mv -f '$home/multirk/multirkd.new' '$home/multirk/multirkd'"
            )
            if (mv.code != 0) error("no se pudo instalar el binario: ${mv.output}")
            onLog("binario instalado")

            val cfg = JSONObject()
                .put("name", deviceName)
                .put("romsRoot", "/roms")
                .put("dataDir", dataDir)
                .put("protocol", "tcp")
                .put("listen", ":47820")
                .put("auth", true)
                .put("httpListen", "")
                .put("scanIntervalSec", 1800)
                .put("stateRefreshSec", 60)
                .put(
                    "multicast",
                    JSONObject()
                        .put("group", "239.255.42.99")
                        .put("port", 47820)
                        .put("intervalSec", 10)
                )
            upload(cfg.toString(2).toByteArray(Charsets.UTF_8), "$dataDir/config.json", 420)
            onLog("configuración escrita")

            val unit = unitFile(home, dataDir)
            upload(unit.toByteArray(Charsets.UTF_8), "/tmp/multirkd.service", 420)

            val hasSystemd = exec("command -v systemctl >/dev/null 2>&1 && echo yes || echo no")
                .output.contains("yes")
            var mode = "manual"
            if (hasSystemd) {
                val r = sudo(
                    "mv /tmp/multirkd.service /etc/systemd/system/multirkd.service && " +
                        "systemctl daemon-reload && " +
                        "systemctl enable multirkd >/dev/null 2>&1; " +
                        "systemctl restart multirkd"
                )
                if (r.code == 0) {
                    mode = "systemd"
                    onLog("servicio systemd activo")
                } else {
                    onLog("systemd falló: ${r.output.take(120)}")
                }
            }
            if (mode == "manual") {
                val script = "#!/bin/sh\n" +
                    "pkill -f multirkd 2>/dev/null || true\n" +
                    "nohup '$home/multirk/multirkd' -config '$dataDir/config.json' " +
                    "> /dev/shm/multirkd.log 2>&1 &\n"
                exec("cat > '$home/multirk/start.sh' <<'EOF'\n$script" + "EOF\nchmod +x '$home/multirk/start.sh'")
                exec("'$home/multirk/start.sh'")
                onLog("arranque manual (start.sh)")
            }

            Thread.sleep(1200)
            return Result(home, mode)
        } finally {
            close()
        }
    }

    fun uninstall(purgeData: Boolean, onLog: (String) -> Unit) {
        connect()
        try {
            onLog("conectado a $host")
            val home = exec("echo \$HOME").output.trim().ifBlank { "/home/$user" }

            onLog("parando el servicio…")
            sudo(
                "systemctl stop multirkd 2>/dev/null; " +
                    "systemctl disable multirkd 2>/dev/null; " +
                    "rm -f /etc/systemd/system/multirkd.service; " +
                    "systemctl daemon-reload 2>/dev/null; true"
            )
            exec("pkill -f multirkd 2>/dev/null || true")

            onLog("borrando binario y servicio…")
            exec("rm -rf '$home/multirk'")
            if (purgeData) {
                onLog("borrando configuración y token…")
                exec("rm -rf '$home/.config/multirk'")
            } else {
                onLog("se conserva ~/.config/multirk (token)")
            }
            onLog("agente desinstalado")
        } finally {
            close()
        }
    }

    private fun unitFile(home: String, dataDir: String): String = """
        [Unit]
        Description=MULTIRK sync agent
        After=network-online.target
        Wants=network-online.target

        [Service]
        Type=simple
        User=$user
        ExecStart=$home/multirk/multirkd -config $dataDir/config.json
        Restart=on-failure
        RestartSec=5
        Nice=10
        IOSchedulingClass=idle
        MemoryMax=64M
        CPUQuota=25%
        ProtectSystem=full
        PrivateTmp=true
        Environment=GOMEMLIMIT=48MiB
        Environment=GOGC=50
        StandardOutput=journal
        StandardError=journal

        [Install]
        WantedBy=multi-user.target
    """.trimIndent() + "\n"

    private fun shq(s: String): String = "'" + s.replace("'", "'\\''") + "'"
}

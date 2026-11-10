package org.multirk.app.data

import org.json.JSONObject
import java.net.DatagramPacket
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.MulticastSocket
import java.net.SocketTimeoutException

object Discovery {
    const val GROUP = "239.255.42.99"
    const val PORT = 47820

    /**
     * Finds consoles on the LAN.
     *
     * The daemon answers a probe with a unicast "announce" sent to the probe's
     * source address and port, so the probe MUST be sent from the same socket
     * that listens. Probes also go straight to every known console: multicast
     * over WiFi is unreliable, unicast is not.
     *
     * Returns as soon as every known console has answered, so a normal scan is
     * a few hundred ms. [timeoutMs] only bounds the case where something is off.
     */
    fun scan(
        known: List<Device> = emptyList(),
        timeoutMs: Long = 1500,
        onFound: (Device) -> Unit = {}
    ): List<Device> {
        val found = LinkedHashMap<String, Device>()
        val group = InetAddress.getByName(GROUP)
        val socket = MulticastSocket(PORT)
        socket.soTimeout = 150
        val msg = JSONObject()
            .put("magic", "MULTIRK1")
            .put("type", "probe")
            .put("id", "android")
            .put("name", "Android")
            .put("ts", System.currentTimeMillis() / 1000)
            .toString()
            .toByteArray(Charsets.UTF_8)

        fun sendProbes() {
            runCatching { socket.send(DatagramPacket(msg, msg.size, group, PORT)) }
            for (d in known) {
                runCatching {
                    socket.send(
                        DatagramPacket(msg, msg.size, InetAddress.getByName(d.host), d.port)
                    )
                }
            }
        }

        try {
            socket.joinGroup(InetSocketAddress(group, PORT), null)

            val start = System.currentTimeMillis()
            val deadline = start + timeoutMs
            val knownHosts = known.map { it.host }.toSet()
            val buf = ByteArray(4096)
            var nextProbe = start
            var probes = 0

            while (System.currentTimeMillis() < deadline) {
                val now = System.currentTimeMillis()
                // UDP can drop: resend a few times while still waiting.
                if (now >= nextProbe && probes < 4) {
                    sendProbes()
                    probes++
                    nextProbe = now + 400
                }
                try {
                    val packet = DatagramPacket(buf, buf.size)
                    socket.receive(packet)
                    val json = JSONObject(String(packet.data, 0, packet.length, Charsets.UTF_8))
                    if (json.optString("magic") != "MULTIRK1") continue
                    if (json.optString("type") != "announce") continue
                    val id = json.optString("id")
                    if (id.isEmpty() || id == "android") continue
                    val isNew = !found.containsKey(id)
                    found[id] = Device(
                        id = id,
                        name = json.optString("name", "Consola"),
                        host = packet.address.hostAddress ?: "",
                        port = json.optInt("port", PORT),
                        model = json.optString("model"),
                        systems = json.optInt("systems"),
                        games = json.optInt("games"),
                        lastSeen = System.currentTimeMillis()
                    )
                    if (isNew) onFound(found.getValue(id))
                } catch (_: SocketTimeoutException) {
                    // keep waiting until the deadline or until everyone answered
                }

                // Everyone we already knew has answered. Keep listening a
                // moment for brand-new consoles, then stop.
                val elapsed = System.currentTimeMillis() - start
                if (knownHosts.isNotEmpty() &&
                    found.values.map { it.host }.toSet().containsAll(knownHosts) &&
                    elapsed >= 700
                ) {
                    break
                }
            }
        } finally {
            runCatching { socket.leaveGroup(InetSocketAddress(group, PORT), null) }
            socket.close()
        }
        return found.values.toList()
    }
}

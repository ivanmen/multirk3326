package org.multirk.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.material3.Text
import androidx.compose.animation.core.animateFloat
import androidx.compose.runtime.getValue
import org.multirk.app.R
import org.multirk.app.data.Device
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * First screen on every launch: connection history plus a button to add a
 * new console. Nothing else is reachable until the mode is chosen.
 */
@Composable
fun HistoryScreen(
    mode: Int,
    devices: List<Device>,
    online: Set<String>?,
    scanning: Boolean,
    selection: List<String>,
    onScan: suspend () -> Unit,
    onRefresh: () -> Unit,
    onAdd: () -> Unit,
    onSelect: (Device) -> Unit,
    onInfo: (Device) -> Unit,
    onBack: () -> Unit
) {
    androidx.activity.compose.BackHandler { onBack() }
    androidx.compose.runtime.LaunchedEffect(Unit) {
        while (true) {
            onScan()
            kotlinx.coroutines.delay(30_000)
        }
    }
    val pulse by androidx.compose.animation.core.rememberInfiniteTransition(label = "glow")
        .animateFloat(
            initialValue = 0f,
            targetValue = 1f,
            animationSpec = androidx.compose.animation.core.infiniteRepeatable(
                androidx.compose.animation.core.tween(1100),
                androidx.compose.animation.core.RepeatMode.Reverse
            ),
            label = "glowPulse"
        )
    androidx.compose.material3.pulltorefresh.PullToRefreshBox(
        isRefreshing = scanning,
        onRefresh = onRefresh,
        modifier = Modifier
            .fillMaxSize()
            .background(CozyBg)
            .systemBarsPadding()
    ) {
    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp, vertical = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Spacer(Modifier.height(8.dp))
        Box(
            Modifier
                .size(88.dp)
                .shadow(6.dp, RoundedCornerShape(26.dp))
                .clip(RoundedCornerShape(26.dp))
                .background(CozyCard)
                .border(2.dp, CozyCardBorder, RoundedCornerShape(26.dp)),
            contentAlignment = Alignment.Center
        ) {
            CozyIcon(R.drawable.cozy_logo, 64.dp)
        }
        Spacer(Modifier.height(12.dp))
        CozyText("MULTIRK", size = 30, color = CozyPrimary, family = BalooXB)
        Spacer(Modifier.height(2.dp))
        CozyText(
            if (mode == 1) "Elige tu consola" else "Elige tus dos consolas",
            size = 18,
            color = CozyInk,
            family = BalooXB
        )
        CozyText("Historial de conexiones", size = 14, color = CozyInkSoft)
        Spacer(Modifier.height(4.dp))
        CozyText(
            if (mode == 1) {
                "Toca tu consola para continuar."
            } else if (selection.isEmpty()) {
                "Toca primero el origen y luego el destino."
            } else if (selection.size == 1) {
                "Origen elegido. Ahora toca el destino."
            } else {
                "Listo: entrando… (toca una para deshacer)"
            },
            size = 13,
            color = CozyPlaceholder,
            align = TextAlign.Center
        )

        Spacer(Modifier.height(20.dp))

        if (devices.isEmpty()) {
            CozyCard(Modifier.fillMaxWidth()) {
                CozyText("Aún no has conectado ninguna consola.", size = 16, color = CozyInk, family = BalooB)
                Spacer(Modifier.height(4.dp))
                CozyText(
                    "Añade una para empezar.",
                    size = 14,
                    color = CozyInkSoft
                )
            }
        } else {
            devices.forEach { d ->
                val checking = online == null || (scanning && d.id !in online)
                val active = online != null && d.id in online
                val dim = if (active || checking) 1f else 0.5f
                val order = selection.indexOf(d.id)
                val picked = order >= 0
                val pickColor = if (order == 0) CozySecondary else CozyPrimary
                CozyCard(
                    Modifier
                        .fillMaxWidth()
                        .padding(bottom = 10.dp)
                        .graphicsLayer { alpha = dim },
                    onClick = if (active) ({ onSelect(d) }) else null,
                    borderColor = when {
                        picked -> pickColor
                        active -> androidx.compose.ui.graphics.lerp(CozyCardBorder, CozySecondary, pulse)
                        else -> CozyCardBorder
                    },
                    borderWidth = when {
                        picked -> 4.dp
                        active -> (2 + pulse).dp
                        else -> 2.dp
                    },
                    elevation = when {
                        picked -> 12.dp
                        active -> (4 + pulse * 10).dp
                        else -> 4.dp
                    },
                    glowColor = when {
                        picked -> pickColor
                        active -> CozySecondary
                        else -> null
                    }
                ) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Box(
                            Modifier
                                .size(50.dp)
                                .clip(RoundedCornerShape(15.dp))
                                .background(CozyField)
                                .border(2.dp, CozyFieldBorder, RoundedCornerShape(15.dp)),
                            contentAlignment = Alignment.Center
                        ) {
                            CozyIcon(R.drawable.cozy_nav_devices, 38.dp)
                        }
                        Spacer(Modifier.width(10.dp))
                        Column(Modifier.weight(1f)) {
                            Text(
                                text = d.name,
                                fontFamily = BalooXB,
                                fontSize = 17.sp,
                                color = if (active || checking) CozyInk else CozyInkSoft,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                            CozyText("${d.host}:${d.port}", size = 13, color = CozyInkSoft)
                            CozyText(lastSeenLabel(d.lastSeen), size = 12, color = CozyPlaceholder)
                        }
                        Spacer(Modifier.width(8.dp))
                        Column(horizontalAlignment = Alignment.End) {
                            if (picked) {
                                CozyPill(if (order == 0) "Origen" else "Destino", pickColor)
                                Spacer(Modifier.height(6.dp))
                            }
                            when {
                                checking -> CozyPill("Buscando…", CozyPlaceholder, filled = false)
                                !active -> CozyPill("Inactiva", CozyPlaceholder)
                                d.paired -> Box(Modifier.clickable { onInfo(d) }) {
                                    CozyPill("Info", CozyInkSoft, filled = false)
                                }
                                else -> Unit
                            }
                        }
                    }
                }
            }
        }

        Spacer(Modifier.height(16.dp))
        CozyButton("Añadir consola nueva", onAdd)
        Spacer(Modifier.height(8.dp))
    }
    }
}

private fun lastSeenLabel(epoch: Long): String {
    if (epoch <= 0) return "Sin conexión previa"
    val f = SimpleDateFormat("dd MMM HH:mm", Locale.getDefault())
    return "Visto " + f.format(Date(epoch))
}

/**
 * Second screen: the app needs to know whether the user will connect one or
 * two consoles before showing the rest of the flow.
 */
@Composable
fun StartScreen(onPick: (Int) -> Unit) {
    Column(
        Modifier
            .fillMaxSize()
            .background(CozyBg)
            .systemBarsPadding()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp, vertical = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Spacer(Modifier.height(18.dp))

        Box(
            Modifier
                .size(104.dp)
                .shadow(6.dp, RoundedCornerShape(28.dp))
                .clip(RoundedCornerShape(28.dp))
                .background(CozyCard)
                .border(2.dp, CozyCardBorder, RoundedCornerShape(28.dp)),
            contentAlignment = Alignment.Center
        ) {
            CozyIcon(R.drawable.cozy_logo, 76.dp)
        }

        Spacer(Modifier.height(16.dp))

        CozyText(
            "MULTIRK",
            size = 32,
            color = CozyPrimary,
            family = BalooXB,
            align = TextAlign.Center
        )
        Spacer(Modifier.height(2.dp))
        CozyText(
            "Tus partidas, en todas tus consolas",
            size = 15,
            color = CozyInkSoft,
            align = TextAlign.Center
        )

        Spacer(Modifier.height(30.dp))

        CozyText(
            "¿Cuántas consolas\nvas a conectar?",
            size = 24,
            color = CozyInk,
            family = BalooXB,
            align = TextAlign.Center
        )
        Spacer(Modifier.height(6.dp))
        CozyText(
            "Elige el modo para preparar la conexión.",
            size = 13,
            color = CozyInkSoft,
            align = TextAlign.Center
        )

        Spacer(Modifier.height(18.dp))

        ChoiceCard(
            icon = R.drawable.cozy_mode_one,
            title = "Una consola",
            subtitle = "Catálogo y controles.",
            accent = CozyPrimary
        ) { onPick(1) }

        Spacer(Modifier.height(14.dp))

        ChoiceCard(
            icon = R.drawable.cozy_mode_two,
            title = "Dos consolas",
            subtitle = "Copiar partidas. Pronto también ROMs.",
            accent = CozySecondary
        ) { onPick(2) }

        Spacer(Modifier.height(26.dp))

        CozyText(
            "Sincronización por WiFi · sin cables",
            size = 12,
            color = CozyPlaceholder,
            align = TextAlign.Center
        )
        Spacer(Modifier.height(8.dp))
    }
}

@Composable
private fun ChoiceCard(
    icon: Int,
    title: String,
    subtitle: String,
    accent: Color,
    onClick: () -> Unit
) {
    val shape = RoundedCornerShape(18.dp)
    val iconShape = RoundedCornerShape(16.dp)
    Row(
        Modifier
            .fillMaxWidth()
            .shadow(4.dp, shape)
            .clip(shape)
            .background(CozyCard)
            .border(2.dp, CozyCardBorder, shape)
            .clickable { onClick() }
            .padding(horizontal = 14.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(
            Modifier
                .size(64.dp)
                .clip(iconShape)
                .background(accent.copy(alpha = 0.12f))
                .border(2.dp, accent.copy(alpha = 0.45f), iconShape),
            contentAlignment = Alignment.Center
        ) {
            CozyIcon(icon, 50.dp)
        }
        Spacer(Modifier.width(14.dp))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.Center) {
            CozyText(title, size = 19, color = CozyInk, family = BalooXB)
            Spacer(Modifier.height(2.dp))
            CozyText(subtitle, size = 13, color = CozyInkSoft)
        }
        Spacer(Modifier.width(8.dp))
        CozyText(">", size = 20, color = accent, family = BalooXB)
    }
}

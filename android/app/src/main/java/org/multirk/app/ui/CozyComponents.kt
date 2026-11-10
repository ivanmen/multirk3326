package org.multirk.app.ui

import android.app.Activity
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.view.WindowCompat
import org.multirk.app.AppViewModel
import org.multirk.app.R
import org.multirk.app.Screen
import kotlin.math.roundToInt

val CozyDanger = Color(0xFFB33A2B)
val CozyDangerDark = Color(0xFF8F2C20)

/** Cream status/navigation bars with dark icons for the whole cozy UI. */
@Composable
fun CozyWindow() {
    val context = LocalContext.current
    val window = (context as? Activity)?.window ?: return
    DisposableEffect(Unit) {
        window.statusBarColor = 0xFFF2E5CC.toInt()
        window.navigationBarColor = 0xFFF2E5CC.toInt()
        WindowCompat.getInsetsController(window, window.decorView).apply {
            isAppearanceLightStatusBars = true
            isAppearanceLightNavigationBars = true
        }
        onDispose {
            window.statusBarColor = 0xFF0A0716.toInt()
            window.navigationBarColor = 0xFF0A0716.toInt()
            WindowCompat.getInsetsController(window, window.decorView).apply {
                isAppearanceLightStatusBars = false
                isAppearanceLightNavigationBars = false
            }
        }
    }
}

@Composable
fun CozyIcon(res: Int, size: Dp = 24.dp, modifier: Modifier = Modifier) {
    Image(
        painter = painterResource(res),
        contentDescription = null,
        modifier = modifier.size(size)
    )
}

@Composable
fun CozyField(
    value: String,
    placeholder: String,
    numeric: Boolean = false,
    secret: Boolean = false,
    lines: Int = 1,
    onChange: (String) -> Unit
) {
    val shape = RoundedCornerShape(13.dp)
    Box(
        Modifier
            .fillMaxWidth()
            .then(if (lines > 1) Modifier.height((24 * lines + 24).dp) else Modifier.height(48.dp))
            .clip(shape)
            .background(CozyField)
            .border(2.dp, CozyFieldBorder, shape)
            .padding(horizontal = 14.dp, vertical = if (lines > 1) 12.dp else 0.dp),
        contentAlignment = if (lines > 1) Alignment.TopStart else Alignment.CenterStart
    ) {
        BasicTextField(
            value = value,
            onValueChange = onChange,
            singleLine = lines <= 1,
            keyboardOptions = if (numeric) {
                KeyboardOptions(keyboardType = KeyboardType.Number)
            } else {
                KeyboardOptions.Default
            },
            visualTransformation = if (secret) {
                PasswordVisualTransformation()
            } else {
                VisualTransformation.None
            },
            textStyle = TextStyle(
                fontFamily = BalooSB,
                fontSize = 17.sp,
                color = CozyInk
            ),
            cursorBrush = SolidColor(CozyPrimary),
            modifier = Modifier.fillMaxWidth()
        )
        if (value.isEmpty()) {
            Text(
                text = placeholder,
                fontFamily = BalooSB,
                fontSize = 15.sp,
                color = CozyPlaceholder
            )
        }
    }
}

@Composable
fun CozyPill(
    text: String,
    color: Color,
    filled: Boolean = true
) {
    Box(
        Modifier
            .clip(RoundedCornerShape(999.dp))
            .background(if (filled) color else Color.Transparent)
            .border(2.dp, color, RoundedCornerShape(999.dp))
            .padding(horizontal = 10.dp, vertical = 3.dp)
    ) {
        Text(
            text = text,
            fontFamily = BalooB,
            fontSize = 12.sp,
            color = if (filled) Color.White else color
        )
    }
}

@Composable
fun CozySection(text: String) {
    Row(
        Modifier.padding(top = 16.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(
            Modifier
                .size(7.dp)
                .clip(CircleShape)
                .background(CozyPrimary)
        )
        Spacer(Modifier.width(8.dp))
        Text(
            text = text,
            fontFamily = BalooB,
            fontSize = 13.sp,
            color = CozyInkSoft
        )
    }
}

@Composable
fun CozySlider(value: Float, onChange: (Float) -> Unit, onFinished: () -> Unit) {
    var widthPx by remember { mutableStateOf(1f) }
    Box(
        Modifier
            .fillMaxWidth()
            .height(34.dp)
            .onSizeChanged { widthPx = it.width.toFloat() }
            .pointerInput(Unit) {
                detectDragGestures(
                    onDragEnd = { onFinished() },
                    onDragCancel = { onFinished() }
                ) { change, _ ->
                    onChange((change.position.x / widthPx).coerceIn(0f, 1f))
                }
            }
            .pointerInput(Unit) {
                detectTapGestures { offset ->
                    onChange((offset.x / widthPx).coerceIn(0f, 1f))
                    onFinished()
                }
            }
    ) {
        Canvas(Modifier.fillMaxSize()) {
            val trackH = 12.dp.toPx()
            val top = (size.height - trackH) / 2f
            val radius = 6.dp.toPx()
            drawRoundRect(
                color = CozyFieldBorder,
                topLeft = Offset(0f, top - 3.dp.toPx()),
                size = Size(size.width, trackH + 6.dp.toPx()),
                cornerRadius = androidx.compose.ui.geometry.CornerRadius(radius + 3.dp.toPx())
            )
            drawRoundRect(
                color = CozyField,
                topLeft = Offset(3.dp.toPx(), top),
                size = Size(size.width - 6.dp.toPx(), trackH),
                cornerRadius = androidx.compose.ui.geometry.CornerRadius(radius)
            )
            drawRoundRect(
                color = CozyPrimary,
                topLeft = Offset(3.dp.toPx(), top),
                size = Size((size.width - 6.dp.toPx()) * value.coerceIn(0f, 1f), trackH),
                cornerRadius = androidx.compose.ui.geometry.CornerRadius(radius)
            )
            val thumbR = 9.dp.toPx()
            val x = 3.dp.toPx() + (size.width - 6.dp.toPx()) * value.coerceIn(0f, 1f)
            drawCircle(CozyInk, thumbR + 2.dp.toPx(), Offset(x, size.height / 2f))
            drawCircle(CozyCard, thumbR, Offset(x, size.height / 2f))
        }
    }
}

@Composable
fun CozyTopBar(
    title: String,
    subtitle: String? = null,
    onBack: (() -> Unit)? = null,
    trailing: @Composable RowScope.() -> Unit = {}
) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(CozyBg)
            .padding(horizontal = 14.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        if (onBack != null) {
            Box(
                Modifier
                    .size(40.dp)
                    .clip(RoundedCornerShape(13.dp))
                    .background(CozyCard)
                    .border(2.dp, CozyCardBorder, RoundedCornerShape(13.dp))
                    .clickable { onBack() },
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = "<",
                    fontFamily = BalooXB,
                    fontSize = 18.sp,
                    color = CozyInk
                )
            }
            Spacer(Modifier.width(10.dp))
        }
        Column(Modifier.weight(1f)) {
            Text(
                text = title,
                fontFamily = BalooXB,
                fontSize = 22.sp,
                color = CozyInk
            )
            if (subtitle != null) {
                Text(
                    text = subtitle,
                    fontFamily = BalooSB,
                    fontSize = 13.sp,
                    color = CozyInkSoft
                )
            }
        }
        trailing()
    }
}

@Composable
fun CozyNavBar(
    screen: Screen,
    onNavigate: (Screen) -> Unit,
    onCatalog: () -> Unit
) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(CozyCard)
            .border(2.dp, CozyCardBorder)
            .padding(horizontal = 6.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.SpaceEvenly,
        verticalAlignment = Alignment.CenterVertically
    ) {
        CozyTab(
            R.drawable.cozy_nav_catalog,
            "Catálogo",
            screen == Screen.Catalog || screen == Screen.Game
        ) { onCatalog() }
        CozyTab(
            R.drawable.cozy_nav_controls,
            "Controles",
            screen == Screen.Controls
        ) { onNavigate(Screen.Controls) }
        CozyTab(
            R.drawable.cozy_nav_settings,
            "Ajustes",
            screen == Screen.Settings
        ) { onNavigate(Screen.Settings) }
    }
}

@Composable
private fun CozyTab(icon: Int, label: String, active: Boolean, onClick: () -> Unit) {
    val shape = RoundedCornerShape(14.dp)
    Column(
        Modifier
            .clip(shape)
            .background(if (active) CozyPrimary.copy(alpha = 0.14f) else Color.Transparent)
            .border(2.dp, if (active) CozyPrimary else Color.Transparent, shape)
            .clickable { onClick() }
            .padding(horizontal = 12.dp, vertical = 5.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        CozyIcon(icon, 26.dp)
        Spacer(Modifier.height(2.dp))
        Text(
            text = label,
            fontFamily = if (active) BalooXB else BalooSB,
            fontSize = 12.sp,
            color = if (active) CozyPrimary else CozyInkSoft
        )
    }
}

@Composable
fun CozyToast(msg: String, onDone: () -> Unit) {
    androidx.compose.runtime.LaunchedEffect(msg) {
        kotlinx.coroutines.delay(2600)
        onDone()
    }
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.BottomCenter) {
        Box(
            Modifier
                .padding(bottom = 92.dp)
                .clip(RoundedCornerShape(13.dp))
                .background(CozyInk)
                .padding(horizontal = 16.dp, vertical = 10.dp)
        ) {
            Text(
                text = msg,
                fontFamily = BalooB,
                fontSize = 14.sp,
                color = CozyBg,
                textAlign = TextAlign.Center
            )
        }
    }
}

@Composable
fun CozyBusy(text: String) {
    Box(
        Modifier
            .fillMaxSize()
            .background(CozyInk.copy(alpha = 0.35f)),
        contentAlignment = Alignment.Center
    ) {
        CozyCard(Modifier.fillMaxWidth(0.8f)) {
            Text(
                text = "MULTIRK",
                fontFamily = BalooXB,
                fontSize = 18.sp,
                color = CozyPrimary
            )
            Spacer(Modifier.height(6.dp))
            Text(
                text = text,
                fontFamily = BalooSB,
                fontSize = 16.sp,
                color = CozyInkSoft
            )
        }
    }
}

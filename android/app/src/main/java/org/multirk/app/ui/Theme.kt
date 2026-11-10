package org.multirk.app.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.foundation.Image
import org.multirk.app.R

val Ink = Color(0xFF0A0716)
val Panel = Color(0xFF1B1338)
val Panel2 = Color(0xFF241A4A)
val Panel3 = Color(0xFF2E2260)
val Line = Color(0xFF3B2C6E)
val Lime = Color(0xFFA8FF60)
val Cyan = Color(0xFF5CE1E6)
val Amber = Color(0xFFFFC94A)
val Pink = Color(0xFFFF6BB5)
val Danger = Color(0xFFFF5D5D)
val TextMain = Color(0xFFF2ECFF)
val Muted = Color(0xFFA99BD6)
val Dim = Color(0xFF6F639C)

val PixelFont = FontFamily(Font(R.font.press_start_2p))
val BodyFont = FontFamily(Font(R.font.vt323))

val Pixel = TextStyle(fontFamily = PixelFont, color = TextMain)
val Body = TextStyle(fontFamily = BodyFont, color = TextMain, fontSize = 18.sp)

@Composable
fun MultirkTheme(content: @Composable () -> Unit) {
    val base = LocalDensity.current
    CompositionLocalProvider(
        LocalDensity provides Density(base.density * 1.25f, base.fontScale)
    ) {
        MaterialTheme(
            colorScheme = darkColorScheme(
                primary = Lime,
                onPrimary = Ink,
                secondary = Cyan,
                onSecondary = Ink,
                background = Ink,
                onBackground = TextMain,
                surface = Panel,
                onSurface = TextMain,
                error = Danger,
                onError = Ink
            ),
            content = content
        )
    }
}

@Composable
fun PxText(
    text: String,
    size: Int = 8,
    color: Color = TextMain,
    modifier: Modifier = Modifier,
    align: TextAlign? = null,
    lineHeight: Int = size + 4
) {
    Text(
        text = text,
        modifier = modifier,
        style = TextStyle(
            fontFamily = PixelFont,
            fontSize = size.sp,
            lineHeight = lineHeight.sp,
            color = color,
            textAlign = align ?: TextAlign.Start
        )
    )
}

@Composable
fun BodyText(
    text: String,
    size: Int = 17,
    color: Color = Muted,
    modifier: Modifier = Modifier,
    align: TextAlign? = null
) {
    Text(
        text = text,
        modifier = modifier,
        style = TextStyle(
            fontFamily = BodyFont,
            fontSize = size.sp,
            lineHeight = (size + 3).sp,
            color = color,
            textAlign = align ?: TextAlign.Start
        )
    )
}

@Composable
fun PixelButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    color: Color = Lime,
    textColor: Color = Ink,
    enabled: Boolean = true,
    small: Boolean = false,
    large: Boolean = false
) {
    Box(modifier) {
        if (enabled) {
            Box(
                Modifier
                    .matchParentSize()
                    .offset(4.dp, 4.dp)
                    .background(Ink)
            )
        }
        Box(
            Modifier
                .fillMaxWidth()
                .background(if (enabled) color else Panel2)
                .border(3.dp, Ink)
                .clickable(enabled = enabled) { onClick() }
                .padding(
                    horizontal = 10.dp,
                    vertical = when {
                        large -> 18.dp
                        small -> 6.dp
                        else -> 11.dp
                    }
                ),
            contentAlignment = Alignment.Center
        ) {
            PxText(
                text,
                size = when {
                    large -> 13
                    small -> 7
                    else -> 8
                },
                color = if (enabled) textColor else Dim,
                align = TextAlign.Center
            )
        }
    }
}

@Composable
fun PixelCard(
    modifier: Modifier = Modifier,
    color: Color = Panel,
    content: @Composable ColumnScope.() -> Unit
) {
    Box(modifier) {
        Box(
            Modifier
                .matchParentSize()
                .offset(4.dp, 4.dp)
                .background(Ink)
        )
        Column(
            Modifier
                .fillMaxWidth()
                .background(color)
                .border(3.dp, Ink)
                .padding(12.dp),
            content = content
        )
    }
}

@Composable
fun PixelPill(
    text: String,
    color: Color = Muted,
    filled: Boolean = false
) {
    Box(
        Modifier
            .background(if (filled) color else Color.Transparent)
            .border(BorderStroke(2.dp, color))
            .padding(horizontal = 6.dp, vertical = 3.dp)
    ) {
        PxText(text, size = 6, color = if (filled) Ink else color)
    }
}

@Composable
fun PixelIcon(res: Int, size: Dp = 20.dp, modifier: Modifier = Modifier) {
    Image(
        painter = painterResource(res),
        contentDescription = null,
        modifier = modifier.size(size)
    )
}

@Composable
fun SectionLabel(text: String) {
    Row(
        modifier = Modifier.padding(top = 12.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(
            Modifier
                .size(7.dp)
                .background(Amber)
        )
        Spacer(Modifier.size(7.dp))
        PxText(text, size = 6, color = Dim)
    }
}

@Composable
fun PixelDivider() {
    Box(
        Modifier
            .fillMaxWidth()
            .height(2.dp)
            .background(Line)
    )
}

@Composable
fun PixelSwitch(checked: Boolean, onChange: (Boolean) -> Unit) {
    Box(
        Modifier
            .size(width = 44.dp, height = 24.dp)
            .background(if (checked) Lime.copy(alpha = 0.25f) else Panel2)
            .border(3.dp, if (checked) Lime else Line)
            .clickable { onChange(!checked) }
            .padding(2.dp)
    ) {
        Box(
            Modifier
                .offset(x = if (checked) 20.dp else 0.dp)
                .size(14.dp)
                .background(if (checked) Lime else Dim)
        )
    }
}

@Composable
fun PixelRow(
    modifier: Modifier = Modifier,
    content: @Composable RowScope.() -> Unit
) {
    Row(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
        content = content
    )
}

fun Modifier.pixelCorners(radius: Dp = 0.dp): Modifier =
    this.clip(RoundedCornerShape(radius))

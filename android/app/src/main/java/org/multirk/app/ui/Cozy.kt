package org.multirk.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.multirk.app.R

// ---------------------------------------------------------------- tokens ----
// Cozy palette from docs: warm paper, terracotta actions, sage secondary.
val CozyBg = Color(0xFFF2E5CC)
val CozyInk = Color(0xFF3A2A1C)
val CozyInkSoft = Color(0xFF7D6650)
val CozyCard = Color(0xFFFFF8EC)
val CozyCardBorder = Color(0xFFE7CFA9)
val CozyPrimary = Color(0xFFC8622F)
val CozyPrimaryDark = Color(0xFFA84E22)
val CozySecondary = Color(0xFF6F8F72)
val CozySecondaryDark = Color(0xFF58745B)
val CozyField = Color(0xFFFFFDF7)
val CozyFieldBorder = Color(0xFFE3C9A6)
val CozyPlaceholder = Color(0xFFB49B7D)
val CozyDisabledBg = Color(0xFFD8C8AD)
val CozyDisabledBorder = Color(0xFFC7B594)
val CozyGhostBorder = Color(0xFFD9BF98)

val BalooXB = FontFamily(Font(R.font.baloo2_extrabold))
val BalooB = FontFamily(Font(R.font.baloo2_bold))
val BalooSB = FontFamily(Font(R.font.baloo2_semibold))
val BalooR = FontFamily(Font(R.font.baloo2_regular))

/** Text helper; keeps the cozy type ramp in one place. */
@Composable
fun CozyText(
    text: String,
    size: Int,
    color: Color = CozyInk,
    family: FontFamily = BalooSB,
    align: TextAlign = TextAlign.Start,
    modifier: Modifier = Modifier
) {
    Text(
        text = text,
        modifier = modifier,
        fontFamily = family,
        fontSize = size.sp,
        lineHeight = (size + 3).sp,
        color = color,
        textAlign = align
    )
}

enum class CozyButtonStyle { Primary, Secondary, Ghost, GhostDanger, Danger }

/**
 * Button with the guide's hard 5 dp bottom edge. It is not Material's
 * elevation: the dark shape is part of the button so it reads "cozy".
 */
@Composable
fun CozyButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    style: CozyButtonStyle = CozyButtonStyle.Primary,
    enabled: Boolean = true
) {
    val shape = RoundedCornerShape(13.dp)
    val face = when (style) {
        CozyButtonStyle.Primary -> CozyPrimary
        CozyButtonStyle.Secondary -> CozySecondary
        CozyButtonStyle.Ghost -> Color.Transparent
        CozyButtonStyle.GhostDanger -> Color.Transparent
        CozyButtonStyle.Danger -> CozyDanger
    }
    val dark = when (style) {
        CozyButtonStyle.Primary -> CozyPrimaryDark
        CozyButtonStyle.Secondary -> CozySecondaryDark
        CozyButtonStyle.Ghost -> CozyGhostBorder
        CozyButtonStyle.GhostDanger -> CozyDanger
        CozyButtonStyle.Danger -> CozyDangerDark
    }
    val content = when (style) {
        CozyButtonStyle.Ghost -> CozyInk
        CozyButtonStyle.GhostDanger -> CozyDanger
        else -> Color.White
    }

    Box(modifier) {
        if (enabled && style != CozyButtonStyle.Ghost && style != CozyButtonStyle.GhostDanger) {
            Box(
                Modifier
                    .matchParentSize()
                    .offset(y = 5.dp)
                    .background(dark, shape)
            )
        }
        Box(
            Modifier
                .fillMaxWidth()
                .height(48.dp)
                .clip(shape)
                .background(if (enabled) face else CozyDisabledBg, shape)
                .border(
                    2.dp,
                    when {
                        !enabled -> CozyDisabledBorder
                        style == CozyButtonStyle.Ghost -> CozyGhostBorder
                        style == CozyButtonStyle.GhostDanger -> CozyDanger
                        else -> Color.Transparent
                    },
                    shape
                )
                .clickable(enabled = enabled) { onClick() },
            contentAlignment = Alignment.Center
        ) {
            Text(
                text = text,
                fontFamily = BalooXB,
                fontSize = 16.sp,
                color = if (enabled) content else CozyInkSoft
            )
        }
    }
}

@Composable
fun CozyCard(
    modifier: Modifier = Modifier,
    onClick: (() -> Unit)? = null,
    borderColor: Color = CozyCardBorder,
    borderWidth: androidx.compose.ui.unit.Dp = 2.dp,
    elevation: androidx.compose.ui.unit.Dp = 4.dp,
    glowColor: Color? = null,
    content: @Composable ColumnScope.() -> Unit
) {
    val shape = RoundedCornerShape(18.dp)
    Column(
        modifier
            .then(
                if (glowColor != null) {
                    Modifier.shadow(
                        elevation,
                        shape,
                        ambientColor = glowColor,
                        spotColor = glowColor
                    )
                } else {
                    Modifier.shadow(elevation, shape)
                }
            )
            .clip(shape)
            .background(CozyCard, shape)
            .border(borderWidth, borderColor, shape)
            .then(if (onClick != null) Modifier.clickable { onClick() } else Modifier)
            .padding(14.dp),
        content = content
    )
}

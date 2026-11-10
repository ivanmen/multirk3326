# Guía de UI Android — estilo “cozy”

Basada en la **pantalla de inicio** del proyecto (`assets/shared/scenes/lobby.tscn`).
Sirve para replicar el estilo en **otra aplicación Android** (XML o Jetpack Compose).

> Fuente de verdad del proyecto: `docs/app_style.md`. Esta guía traduce esos
> tokens a Android (colores, `dp`/`sp`, drawables y estilos).

---

## 1. Tokens

### 1.1 Colores — `res/values/colors.xml`

```xml
<resources>
    <color name="bg">#F2E5CC</color>            <!-- fondo -->
    <color name="ink">#3A2A1C</color>           <!-- texto principal -->
    <color name="ink_soft">#7D6650</color>      <!-- etiquetas / secundario -->
    <color name="card">#FFF8EC</color>          <!-- tarjetas -->
    <color name="card_border">#E7CFA9</color>   <!-- borde de tarjeta -->
    <color name="primary">#C8622F</color>       <!-- acción principal -->
    <color name="primary_dark">#A84E22</color>  <!-- borde inferior / sombra dura -->
    <color name="secondary">#6F8F72</color>     <!-- acción secundaria / selectores -->
    <color name="secondary_dark">#58745B</color><!-- borde inferior del secundario -->
    <color name="field">#FFFDF7</color>         <!-- fondo de inputs -->
    <color name="field_border">#E3C9A6</color>  <!-- borde de inputs -->
    <color name="placeholder">#B49B7D</color>   <!-- placeholder -->
    <color name="disabled_bg">#D8C8AD</color>   <!-- deshabilitado -->
    <color name="disabled_border">#C7B594</color>
    <color name="ghost_border">#D9BF98</color>  <!-- botón fantasma -->
    <color name="on_primary">#FFFFFF</color>    <!-- texto sobre primario -->
</resources>
```

### 1.2 Dimensiones — `res/values/dimens.xml`

La pantalla del proyecto es 360 × 640; en Android usa **1 unidad ≈ 1 dp** para una
pantalla de 360 dp de ancho.

```xml
<resources>
    <dimen name="radius_button">13dp</dimen>
    <dimen name="radius_card">18dp</dimen>
    <dimen name="radius_input">13dp</dimen>
    <dimen name="radius_popup">12dp</dimen>
    <dimen name="stroke">2dp</dimen>
    <dimen name="button_depth">5dp</dimen>
    <dimen name="button_height">44dp</dimen>
    <dimen name="input_height">44dp</dimen>
    <dimen name="space">12dp</dimen>
    <dimen name="card_padding_h">14dp</dimen>
    <dimen name="card_padding_v">11dp</dimen>
</resources>
```

### 1.3 Tipografía

- **Baloo 2** (Google Fonts, OFL). El proyecto usa instancias estáticas:
  `Baloo2-ExtraBold.ttf`, `Baloo2-Bold.ttf`, `Baloo2-SemiBold.ttf`
  (en `assets/shared/fonts/`).
- Cópialas a `res/font/` (minúsculas y guiones bajos):
  `baloo2_extrabold.ttf`, `baloo2_bold.ttf`, `baloo2_semibold.ttf`.
- Tamaños: título **25sp**, títulos de tarjeta **14sp**, etiquetas/estado **13sp**,
  inputs **16sp**, botones **16sp**.

```xml
<!-- res/values/styles.xml (tipografía) -->
<style name="Text.Title" parent="android:Widget.TextView">
    <item name="android:fontFamily">@font/baloo2_extrabold</item>
    <item name="android:textSize">25sp</item>
    <item name="android:textColor">@color/ink</item>
</style>
<style name="Text.CardTitle" parent="android:Widget.TextView">
    <item name="android:fontFamily">@font/baloo2_extrabold</item>
    <item name="android:textSize">14sp</item>
    <item name="android:textColor">@color/ink</item>
</style>
<style name="Text.Label" parent="android:Widget.TextView">
    <item name="android:fontFamily">@font/baloo2_bold</item>
    <item name="android:textSize">13sp</item>
    <item name="android:textColor">@color/ink_soft</item>
</style>
<style name="Text.Body" parent="android:Widget.TextView">
    <item name="android:fontFamily">@font/baloo2_semibold</item>
    <item name="android:textSize">13sp</item>
    <item name="android:textColor">@color/ink_soft</item>
</style>
```

---

## 2. Componentes

### 2.1 Botón primario (terracota, profundidad dura)

El “cozy” usa un **borde inferior sólido** de 5 dp (no blur). En Android se logra
con un `layer-list`: una forma oscura de fondo y la forma principal encima con
`android:bottom="5dp"`.

`res/drawable/bg_btn_primary.xml`

```xml
<layer-list xmlns:android="http://schemas.android.com/apk/res/android">
    <!-- borde inferior / sombra dura -->
    <item>
        <shape android:shape="rectangle">
            <solid android:color="@color/primary_dark"/>
            <corners android:radius="@dimen/radius_button"/>
        </shape>
    </item>
    <!-- cara del botón, levantada 5dp -->
    <item android:bottom="@dimen/button_depth">
        <shape android:shape="rectangle">
            <solid android:color="@color/primary"/>
            <corners android:radius="@dimen/radius_button"/>
        </shape>
    </item>
</layer-list>
```

`res/drawable/bg_btn_primary_pressed.xml` (se “hunde”: borde de 2 dp y tono oscuro)

```xml
<layer-list xmlns:android="http://schemas.android.com/apk/res/android">
    <item>
        <shape android:shape="rectangle">
            <solid android:color="@color/primary_dark"/>
            <corners android:radius="@dimen/radius_button"/>
        </shape>
    </item>
    <item android:bottom="2dp">
        <shape android:shape="rectangle">
            <solid android:color="@color/primary_dark"/>
            <corners android:radius="@dimen/radius_button"/>
        </shape>
    </item>
</layer-list>
```

`res/drawable/bg_btn_disabled.xml`

```xml
<shape xmlns:android="http://schemas.android.com/apk/res/android" android:shape="rectangle">
    <solid android:color="@color/disabled_bg"/>
    <stroke android:width="@dimen/stroke" android:color="@color/disabled_border"/>
    <corners android:radius="@dimen/radius_button"/>
</shape>
```

`res/drawable/btn_primary.xml` (selector de estados)

```xml
<selector xmlns:android="http://schemas.android.com/apk/res/android">
    <item android:state_enabled="false" android:drawable="@drawable/bg_btn_disabled"/>
    <item android:state_pressed="true" android:drawable="@drawable/bg_btn_primary_pressed"/>
    <item android:drawable="@drawable/bg_btn_primary"/>
</selector>
```

Estilo:

```xml
<style name="Widget.App.Button.Primary" parent="android:Widget.Material.Button">
    <item name="android:background">@drawable/btn_primary</item>
    <item name="android:textColor">@color/on_primary</item>
    <item name="android:fontFamily">@font/baloo2_extrabold</item>
    <item name="android:textSize">16sp</item>
    <item name="android:textAllCaps">false</item>
    <item name="android:minHeight">@dimen/button_height</item>
    <item name="android:paddingLeft">18dp</item>
    <item name="android:paddingRight">18dp</item>
    <item name="android:paddingTop">9dp</item>
    <item name="android:paddingBottom">14dp</item>
</style>
```

Uso:

```xml
<Button
    android:id="@+id/btnCreate"
    style="@style/Widget.App.Button.Primary"
    android:layout_width="match_parent"
    android:layout_height="wrap_content"
    android:text="Crear sala" />
```

### 2.2 Botón secundario (salvia)

Igual que el primario, cambiando `primary` por `secondary` y `primary_dark` por
`secondary_dark`. Reutiliza la misma estructura de `layer-list` con
`@color/secondary` y `@color/secondary_dark`, y crea
`btn_secondary.xml` con sus estados.

### 2.3 Botón fantasma (contorno)

`res/drawable/bg_btn_ghost.xml`

```xml
<shape xmlns:android="http://schemas.android.com/apk/res/android" android:shape="rectangle">
    <solid android:color="@android:color/transparent"/>
    <stroke android:width="@dimen/stroke" android:color="@color/ghost_border"/>
    <corners android:radius="@dimen/radius_button"/>
</shape>
```

`bg_btn_ghost_pressed.xml` añade un relleno claro (`#66FFFFFF`).
Estilo con `textColor=@color/ink`, `fontFamily=baloo2_extrabold`, 14sp.

### 2.4 Botón de información (icono)

- Icono del proyecto (PixelLab): `games/serpientes_escaleras/assets/ui/info_icon.png`
  (64×64, transparente). Cópialo a `res/drawable/info_icon.png`.
- Botón plano, sin fondo, 40–48 dp, icono centrado:

```xml
<ImageButton
    android:id="@+id/btnInfo"
    android:layout_width="44dp"
    android:layout_height="44dp"
    android:background="@android:color/transparent"
    android:contentDescription="Información"
    android:scaleType="fitCenter"
    android:src="@drawable/info_icon" />
```

Si no usas icono, un `Button` circular con “i” y `background=bg_btn_info.xml`
(forma crema 92 %, borde `card_border`, radio 20 dp).

### 2.5 Input de texto

`res/drawable/bg_input.xml`

```xml
<shape xmlns:android="http://schemas.android.com/apk/res/android" android:shape="rectangle">
    <solid android:color="@color/field"/>
    <stroke android:width="@dimen/stroke" android:color="@color/field_border"/>
    <corners android:radius="@dimen/radius_input"/>
</shape>
```

`res/drawable/bg_input_focus.xml` igual pero con `stroke` en `@color/primary`.
Selector `input_bg.xml` con `state_focused` → focus.

```xml
<EditText
    android:id="@+id/nameEdit"
    android:layout_width="match_parent"
    android:layout_height="44dp"
    android:background="@drawable/input_bg"
    android:fontFamily="@font/baloo2_semibold"
    android:hint="Jugador"
    android:paddingLeft="13dp"
    android:paddingRight="13dp"
    android:textColor="@color/ink"
    android:textColorHint="@color/placeholder"
    android:textSize="16sp" />
```

### 2.6 ComboBox (Spinner) estilo salvia

- Fondo del selector: `@drawable/btn_secondary` (o una forma salvia con radio 13 dp).
- Texto blanco (`on_primary`), Baloo Bold 13sp.
- **Popup** crema con borde y radio 12 dp:

`res/drawable/bg_popup.xml`

```xml
<shape xmlns:android="http://schemas.android.com/apk/res/android" android:shape="rectangle">
    <solid android:color="@color/card"/>
    <stroke android:width="@dimen/stroke" android:color="@color/card_border"/>
    <corners android:radius="@dimen/radius_popup"/>
</shape>
```

`res/drawable/bg_popup_item.xml` (hover/pressed terracota 16 %)

```xml
<selector xmlns:android="http://schemas.android.com/apk/res/android">
    <item android:state_pressed="true">
        <shape android:shape="rectangle">
            <solid android:color="#29C8622F"/> <!-- terracota 16% -->
            <corners android:radius="8dp"/>
        </shape>
    </item>
    <item android:drawable="@android:color/transparent"/>
</selector>
```

`res/layout/spinner_item.xml` (texto cerrado)

```xml
<TextView xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="wrap_content"
    android:paddingLeft="12dp"
    android:paddingRight="12dp"
    android:paddingTop="9dp"
    android:paddingBottom="9dp"
    android:fontFamily="@font/baloo2_bold"
    android:textColor="@color/on_primary"
    android:textSize="13sp" />
```

`res/layout/spinner_dropdown_item.xml` (ítem del popup)

```xml
<TextView xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="wrap_content"
    android:background="@drawable/bg_popup_item"
    android:paddingLeft="12dp"
    android:paddingRight="12dp"
    android:paddingTop="8dp"
    android:paddingBottom="8dp"
    android:fontFamily="@font/baloo2_semibold"
    android:textColor="@color/ink"
    android:textSize="15sp" />
```

```xml
<Spinner
    android:id="@+id/gameSpinner"
    android:layout_width="0dp"
    android:layout_height="34dp"
    android:layout_weight="1"
    android:background="@drawable/btn_secondary"
    android:popupBackground="@drawable/bg_popup"
    android:spinnerMode="dropdown" />
```

### 2.7 RadioButton / RadioGroup

Círculo salvia/crema con punto terracota:

`res/drawable/btn_radio.xml`

```xml
<selector xmlns:android="http://schemas.android.com/apk/res/android">
    <item android:state_checked="true">
        <shape android:shape="oval">
            <solid android:color="@color/primary"/>
            <stroke android:width="@dimen/stroke" android:color="@color/primary_dark"/>
            <size android:width="20dp" android:height="20dp"/>
        </shape>
    </item>
    <item>
        <shape android:shape="oval">
            <solid android:color="@android:color/transparent"/>
            <stroke android:width="@dimen/stroke" android:color="@color/ghost_border"/>
            <size android:width="20dp" android:height="20dp"/>
        </shape>
    </item>
</selector>
```

```xml
<RadioGroup
    android:id="@+id/playersGroup"
    android:layout_width="match_parent"
    android:layout_height="wrap_content"
    android:orientation="vertical">

    <RadioButton
        android:id="@+id/players2"
        android:layout_width="match_parent"
        android:layout_height="44dp"
        android:button="@drawable/btn_radio"
        android:fontFamily="@font/baloo2_semibold"
        android:paddingLeft="8dp"
        android:text="2 jugadores"
        android:textColor="@color/ink"
        android:textSize="15sp" />
</RadioGroup>
```

### 2.8 Tarjetas

`res/drawable/bg_card.xml`

```xml
<shape xmlns:android="http://schemas.android.com/apk/res/android" android:shape="rectangle">
    <solid android:color="@color/card"/>
    <stroke android:width="@dimen/stroke" android:color="@color/card_border"/>
    <corners android:radius="@dimen/radius_card"/>
</shape>
```

```xml
<LinearLayout
    android:layout_width="match_parent"
    android:layout_height="wrap_content"
    android:background="@drawable/bg_card"
    android:elevation="4dp"
    android:orientation="vertical"
    android:paddingLeft="@dimen/card_padding_h"
    android:paddingTop="@dimen/card_padding_v"
    android:paddingRight="@dimen/card_padding_h"
    android:paddingBottom="@dimen/card_padding_v" />
```

### 2.9 Diálogo / overlay de ayuda

- Fondo oscurecido `#8C1A1610` (60 % de `#1A1610`).
- Tarjeta centrada con `bg_card`.
- Título Baloo ExtraBold 20sp, cuerpo SemiBold 13sp.
- Botón primario “Entendido”.

Puedes usar un `Dialog` con `android:windowBackground="@android:color/transparent"` y
una tarjeta inflada, o un `FrameLayout` overlay en la propia pantalla.

---

## 3. Pantalla de ejemplo (XML)

```xml
<ScrollView xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    android:background="@color/bg"
    android:fillViewport="true">

    <LinearLayout
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:orientation="vertical"
        android:padding="16dp">

        <!-- Logo 96×96: res/drawable/app_logo.png -->
        <ImageView
            android:layout_width="96dp"
            android:layout_height="96dp"
            android:layout_gravity="center_horizontal"
            android:contentDescription="@null"
            android:src="@drawable/app_logo" />

        <TextView
            style="@style/Text.Title"
            android:layout_width="match_parent"
            android:layout_height="wrap_content"
            android:layout_marginTop="8dp"
            android:gravity="center"
            android:text="Jugar en línea" />

        <TextView
            style="@style/Text.Label"
            android:layout_width="match_parent"
            android:layout_height="wrap_content"
            android:layout_marginTop="4dp"
            android:gravity="center"
            android:text="" />

        <TextView
            style="@style/Text.Label"
            android:layout_width="wrap_content"
            android:layout_height="wrap_content"
            android:layout_marginTop="12dp"
            android:text="Tu nombre" />

        <EditText
            android:id="@+id/nameEdit"
            android:layout_width="match_parent"
            android:layout_height="44dp"
            android:layout_marginTop="6dp"
            android:background="@drawable/input_bg"
            android:fontFamily="@font/baloo2_semibold"
            android:hint="Jugador"
            android:paddingLeft="13dp"
            android:paddingRight="13dp"
            android:textColor="@color/ink"
            android:textColorHint="@color/placeholder"
            android:textSize="16sp" />

        <LinearLayout
            android:layout_width="match_parent"
            android:layout_height="wrap_content"
            android:layout_marginTop="12dp"
            android:background="@drawable/bg_card"
            android:elevation="4dp"
            android:orientation="vertical"
            android:padding="14dp">

            <TextView style="@style/Text.CardTitle"
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:text="Puedes crear una sala" />

            <LinearLayout
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:layout_marginTop="8dp"
                android:gravity="center_vertical"
                android:orientation="horizontal">

                <TextView style="@style/Text.Label"
                    android:layout_width="0dp"
                    android:layout_height="wrap_content"
                    android:layout_weight="1"
                    android:text="Jugadores" />

                <Spinner
                    android:id="@+id/playersSpinner"
                    android:layout_width="104dp"
                    android:layout_height="34dp"
                    android:background="@drawable/btn_secondary"
                    android:popupBackground="@drawable/bg_popup" />
            </LinearLayout>

            <LinearLayout
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:layout_marginTop="8dp"
                android:gravity="center_vertical"
                android:orientation="horizontal">

                <TextView style="@style/Text.Label"
                    android:layout_width="0dp"
                    android:layout_height="wrap_content"
                    android:layout_weight="1"
                    android:text="Juego" />

                <Spinner
                    android:id="@+id/gameSpinner"
                    android:layout_width="176dp"
                    android:layout_height="34dp"
                    android:background="@drawable/btn_secondary"
                    android:popupBackground="@drawable/bg_popup" />
            </LinearLayout>

            <Button
                android:id="@+id/btnCreate"
                style="@style/Widget.App.Button.Primary"
                android:layout_width="match_parent"
                android:layout_height="44dp"
                android:layout_marginTop="10dp"
                android:text="Crear sala" />
        </LinearLayout>

        <LinearLayout
            android:layout_width="match_parent"
            android:layout_height="wrap_content"
            android:layout_marginTop="12dp"
            android:background="@drawable/bg_card"
            android:elevation="4dp"
            android:orientation="vertical"
            android:padding="14dp">

            <TextView style="@style/Text.CardTitle"
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:text="O puedes unirte a una sala" />

            <LinearLayout
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:layout_marginTop="8dp"
                android:orientation="horizontal">

                <EditText
                    android:id="@+id/codeEdit"
                    android:layout_width="0dp"
                    android:layout_height="44dp"
                    android:layout_weight="1"
                    android:background="@drawable/input_bg"
                    android:fontFamily="@font/baloo2_extrabold"
                    android:gravity="center"
                    android:hint="ABCDE"
                    android:inputType="textCapCharacters"
                    android:maxLength="5"
                    android:textColor="@color/ink"
                    android:textColorHint="@color/placeholder"
                    android:textSize="18sp" />

                <Button
                    android:id="@+id/btnJoin"
                    style="@style/Widget.App.Button.Secondary"
                    android:layout_width="100dp"
                    android:layout_height="44dp"
                    android:layout_marginLeft="10dp"
                    android:text="Unirse" />
            </LinearLayout>
        </LinearLayout>
    </LinearLayout>
</ScrollView>
```

---

## 4. Equivalentes en Jetpack Compose

```kotlin
val Bg = Color(0xFFF2E5CC)
val Ink = Color(0xFF3A2A1C)
val InkSoft = Color(0xFF7D6650)
val Card = Color(0xFFFFF8EC)
val CardBorder = Color(0xFFE7CFA9)
val Primary = Color(0xFFC8622F)
val PrimaryDark = Color(0xFFA84E22)
val Secondary = Color(0xFF6F8F72)
val SecondaryDark = Color(0xFF58745B)
val Field = Color(0xFFFFFDF7)
val FieldBorder = Color(0xFFE3C9A6)
val Placeholder = Color(0xFFB49B7D)
val GhostBorder = Color(0xFFD9BF98)

val BalooXB = FontFamily(Font(R.font.baloo2_extrabold))
val BalooB = FontFamily(Font(R.font.baloo2_bold))
val BalooSB = FontFamily(Font(R.font.baloo2_semibold))

// Botón primario
Button(
    onClick = { },
    shape = RoundedCornerShape(13.dp),
    colors = ButtonDefaults.buttonColors(
        containerColor = Primary,
        contentColor = Color.White
    ),
    elevation = ButtonDefaults.buttonElevation(defaultElevation = 0.dp, pressedElevation = 0.dp),
    modifier = Modifier
        .fillMaxWidth()
        .height(44.dp)
) {
    Text("Crear sala", fontFamily = BalooXB, fontSize = 16.sp)
}

// Tarjeta
Card(
    shape = RoundedCornerShape(18.dp),
    colors = CardDefaults.cardColors(containerColor = Card),
    border = BorderStroke(2.dp, CardBorder),
    elevation = CardDefaults.cardElevation(defaultElevation = 4.dp)
) { /* contenido */ }

// Input
OutlinedTextField(
    value = name,
    onValueChange = { name = it },
    placeholder = { Text("Jugador", color = Placeholder) },
    textStyle = TextStyle(fontFamily = BalooSB, fontSize = 16.sp, color = Ink),
    shape = RoundedCornerShape(13.dp),
    colors = OutlinedTextFieldDefaults.colors(
        focusedBorderColor = Primary,
        unfocusedBorderColor = FieldBorder,
        focusedContainerColor = Field,
        unfocusedContainerColor = Field
    ),
    modifier = Modifier.fillMaxWidth()
)

// ComboBox
ExposedDropdownMenuBox(expanded = expanded, onExpandedChange = { expanded = !expanded }) {
    OutlinedTextField(
        value = selected,
        onValueChange = {},
        readOnly = true,
        shape = RoundedCornerShape(13.dp),
        colors = OutlinedTextFieldDefaults.colors(
            focusedContainerColor = Secondary,
            unfocusedContainerColor = Secondary,
            focusedTextColor = Color.White,
            unfocusedTextColor = Color.White,
            focusedBorderColor = SecondaryDark,
            unfocusedBorderColor = SecondaryDark
        ),
        trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded) },
        modifier = Modifier.menuAnchor().height(44.dp)
    )
    ExposedDropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
        options.forEach { option ->
            DropdownMenuItem(
                text = { Text(option, fontFamily = BalooSB, color = Ink) },
                onClick = { selected = option; expanded = false }
            )
        }
    }
}

// RadioButton
RadioButton(
    selected = checked,
    onClick = { checked = true },
    colors = RadioButtonDefaults.colors(
        selectedColor = Primary,
        unselectedColor = GhostBorder
    )
)
```

---

## 5. Assets y fuentes

- Fuentes Baloo 2: `assets/shared/fonts/Baloo2-*.ttf`
  → copiar a `res/font/` (minúsculas, guiones bajos).
- Logo de pantalla: `assets/shared/ui/app_logo.png` (96×96).
- Icono de app: `assets/shared/ui/app_icon.png`.
- Icono de información (PixelLab): `games/serpientes_escaleras/assets/ui/info_icon.png`
  (64×64, transparente).

## 6. Notas

- **Área táctil:** el proyecto usa 44 dp; en Android conviene **48 dp** como mínimo
  accesible. Ajusta `minHeight`/`layout_height` a 48 dp si puedes.
- **Sombra dura:** el borde inferior de 5 dp es parte del estilo. Si prefieres algo
  más nativo, usa `elevation` (4 dp) y elimina el `layer-list`.
- **Estados:** siempre define `pressed` y `disabled` con selectores; no dejes el
  fondo por defecto de Material, que rompe la paleta.
- **Texto:** usa `textAllCaps=false` en los botones para conservar “Crear sala”.
- **Contraste:** `on_primary` blanco sobre `primary` cumple; evita poner `ink`
  sobre `primary`.

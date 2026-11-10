package org.multirk.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.viewModels
import org.multirk.app.ui.MultirkApp

class MainActivity : ComponentActivity() {

    private val vm: AppViewModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MultirkApp(vm)
        }
        intent.getStringExtra("host")?.let { host ->
            val port = intent.getIntExtra("port", 47820)
            vm.addManual(host, port.toString())
        }
    }
}

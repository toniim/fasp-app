import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import PinWindow from './components/Pin/PinWindow'
import { Call } from '@wailsio/runtime'

// A Go error rejects the call with a RuntimeError. Toasts print errors as
// `${error}`; keep that reading as the bare Go message, without the
// "RuntimeError: " prefix.
Call.RuntimeError.prototype.toString = function () {
  return this.message
}

const container = document.getElementById('root')

const root = createRoot(container!)

// A pinned image is a separate window that loads this same page with ?pin=<id>.
const pinId = new URLSearchParams(window.location.search).get('pin')

root.render(
    <React.StrictMode>
        {pinId ? <PinWindow id={pinId}/> : <App/>}
    </React.StrictMode>
)

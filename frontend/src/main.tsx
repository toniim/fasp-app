import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import { Call } from '@wailsio/runtime'

// A Go error rejects the call with a RuntimeError. Toasts print errors as
// `${error}`; keep that reading as the bare Go message, without the
// "RuntimeError: " prefix.
Call.RuntimeError.prototype.toString = function () {
  return this.message
}

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)

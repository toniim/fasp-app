import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import PinWindow from './components/Pin/PinWindow'
import RecordDialog from './components/Recording/RecordDialog'
import { Call } from '@wailsio/runtime'

// A Go error rejects the call with a RuntimeError. Toasts print errors as
// `${error}`; keep that reading as the bare Go message, without the
// "RuntimeError: " prefix.
Call.RuntimeError.prototype.toString = function () {
  return this.message
}

const container = document.getElementById('root')

const root = createRoot(container!)

// Other windows load this same page: a pinned image with ?pin=<id>, the
// recording options with ?view=record.
const params = new URLSearchParams(window.location.search)
const pinId = params.get('pin')
const view = params.get('view')
// The record window shows the dialog's own background around it while it is
// being fitted to the content.
if (view === 'record') document.documentElement.classList.add('record-page')

root.render(
    <React.StrictMode>
        {pinId ? <PinWindow id={pinId}/> : view === 'record' ? <RecordDialog/> : <App/>}
    </React.StrictMode>
)

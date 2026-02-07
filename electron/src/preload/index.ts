import { contextBridge, ipcRenderer } from 'electron'

// Custom APIs for renderer
const api = {
    send: (channel: string, data: any) => {
        // Whitelist channels? For now, we only use 'to-python'
        if (channel === 'to-python') {
            ipcRenderer.send(channel, data);
        }
    },
    on: (channel: string, func: (...args: any[]) => void) => {
        if (channel === 'python-event') {
            const subscription = (_event: any, ...args: any[]) => func(...args);
            ipcRenderer.on(channel, subscription);
            return () => ipcRenderer.removeListener(channel, subscription);
        }
        return () => { };
    }
}

// Use `contextBridge` APIs to expose functionality to the renderer
// Read more on https://www.electronjs.org/docs/latest/tutorial/context-isolation
try {
    contextBridge.exposeInMainWorld('electron', api)
} catch (error) {
    console.error(error)
}

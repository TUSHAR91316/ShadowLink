import { app, shell, BrowserWindow, ipcMain } from 'electron'
import { join } from 'path'
import { electronApp, optimizer, is } from '@electron-toolkit/utils'
import { spawn, ChildProcess } from 'child_process'
import path from 'path'

let pythonProcess: ChildProcess | null = null;
let mainWindow: BrowserWindow | null = null;

function createWindow(): void {
    // Create the browser window.
    mainWindow = new BrowserWindow({
        width: 900,
        height: 670,
        show: false,
        autoHideMenuBar: true,
        ...(process.platform === 'linux' ? { icon: join(__dirname, '../../build/icon.png') } : {}),
        webPreferences: {
            preload: join(__dirname, '../preload/index.js'),
            sandbox: false
        }
    })

    mainWindow.on('ready-to-show', () => {
        mainWindow?.show()
    })

    mainWindow.webContents.setWindowOpenHandler((details) => {
        shell.openExternal(details.url)
        return { action: 'deny' }
    })

    // HMR for renderer base on electron-vite cli.
    // Load the remote URL for development or the local html file for production.
    if (is.dev && process.env['ELECTRON_RENDERER_URL']) {
        mainWindow.loadURL(process.env['ELECTRON_RENDERER_URL'])
    } else {
        mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
    }
}

// --- Python Process Management ---

function getPythonPath(): string {
    if (is.dev) {
        // In dev, use python from env and point to src/api.py
        // Assuming we are in electron/ directory, python script is in ../src/api.py
        return 'python'
    } else {
        // In prod, use bundled executable
        return path.join(process.resourcesPath, 'api.exe') // or whatever name we give it
    }
}

function getScriptPath(): string[] {
    if (is.dev) {
        return [path.join(__dirname, '../../../src/api.py')]
    }
    return []
}

function startPython() {
    const exe = getPythonPath();
    const args = getScriptPath();

    console.log(`Starting Python: ${exe} ${args.join(' ')}`)

    pythonProcess = spawn(exe, args);

    if (pythonProcess.stdout) {
        pythonProcess.stdout.on('data', (data) => {
            const str = data.toString().trim();
            // Could be multiple JSONs or partial
            str.split('\n').forEach((line: string) => {
                if (!line) return;
                try {
                    const json = JSON.parse(line);
                    // Send to renderer
                    mainWindow?.webContents.send('python-event', json);
                    console.log('Python Event:', json);
                } catch (e) {
                    console.log('Python Stdout (text):', line);
                }
            });
        });
    }

    if (pythonProcess.stderr) {
        pythonProcess.stderr.on('data', (data) => {
            console.error(`Python Stderr: ${data}`);
        });
    }

    pythonProcess.on('close', (code) => {
        console.log(`Python process exited with code ${code}`);
        mainWindow?.webContents.send('python-event', { type: 'status', data: { state: 'stopped' } });
    });
}

function stopPython() {
    if (pythonProcess) {
        // Send quit command first?
        sendCommand({ cmd: 'quit' });
        // Give time then kill
        setTimeout(() => {
            pythonProcess?.kill();
            pythonProcess = null;
        }, 1000);
    }
}

function sendCommand(cmd: any) {
    if (pythonProcess && pythonProcess.stdin) {
        pythonProcess.stdin.write(JSON.stringify(cmd) + '\n');
    }
}

// --- App Lifecycle ---

app.whenReady().then(() => {
    // Set app user model id for windows
    electronApp.setAppUserModelId('com.shadowlink')

    // Default open or close DevTools by F12 in development
    // and ignore CommandOrControl + R in production.
    // see https://github.com/alex8088/electron-toolkit/tree/master/packages/utils
    app.on('browser-window-created', (_, window) => {
        optimizer.watchWindowShortcuts(window)
    })

    // IPC Handlers
    ipcMain.on('to-python', (event, args) => {
        sendCommand(args);
    });

    startPython();
    createWindow();

    app.on('activate', function () {
        // On macOS it's common to re-create a window in the app when the
        // dock icon is clicked and there are no other windows open.
        if (BrowserWindow.getAllWindows().length === 0) createWindow()
    })
})

app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') {
        stopPython();
        app.quit()
    }
})

app.on('before-quit', () => {
    stopPython();
})

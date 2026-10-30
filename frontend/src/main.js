// State Management
let currentLocalPath = "";
let currentRemotePath = "~";
let selectedLocalItem = null;
let selectedRemoteItem = null;
let clipboard = null; // { action: 'cut'|'copy', pane: 'local'|'remote', item: FileItem }
let savedProfiles = [];
let selectedProfileId = null;
let activePane = "local"; // 'local' or 'remote'
let terminalPollingInterval = null;

// DOM Elements
const el = {
    // Buttons
    btnManager: document.getElementById("btnManager"),
    btnUp: document.getElementById("btnUp"),
    btnCut: document.getElementById("btnCut"),
    btnCopy: document.getElementById("btnCopy"),
    btnPaste: document.getElementById("btnPaste"),
    btnDelete: document.getElementById("btnDelete"),
    btnRename: document.getElementById("btnRename"),
    btnRefresh: document.getElementById("btnRefresh"),
    btnNewFolder: document.getElementById("btnNewFolder"),
    btnNewText: document.getElementById("btnNewText"),
    btnChmod: document.getElementById("btnChmod"),
    btnDisconnect: document.getElementById("btnDisconnect"),
    connStatus: document.getElementById("connStatus"),

    // Local Pane
    localPathInput: document.getElementById("localPathInput"),
    btnLocalGo: document.getElementById("btnLocalGo"),
    localFileTableBody: document.getElementById("localFileTableBody"),
    localCount: document.getElementById("localCount"),

    // Remote Pane
    remotePathInput: document.getElementById("remotePathInput"),
    btnRemoteGo: document.getElementById("btnRemoteGo"),
    remoteFileTableBody: document.getElementById("remoteFileTableBody"),
    remoteCount: document.getElementById("remoteCount"),

    // Terminal
    splitterH: document.getElementById("splitterH"),
    filePanesContainer: document.getElementById("filePanesContainer"),
    terminalContainer: document.getElementById("terminalContainer"),
    terminalOutput: document.getElementById("terminalOutput"),
    terminalPrompt: document.getElementById("terminalPrompt"),
    terminalInput: document.getElementById("terminalInput"),

    // Side Editors
    localEditorToast: document.getElementById("localEditorToast"),
    localEditorPath: document.getElementById("localEditorPath"),
    localEditorText: document.getElementById("localEditorText"),
    btnLocalEditorSave: document.getElementById("btnLocalEditorSave"),
    btnLocalEditorHide: document.getElementById("btnLocalEditorHide"),
    btnLocalEditorClose: document.getElementById("btnLocalEditorClose"),

    remoteEditorToast: document.getElementById("remoteEditorToast"),
    remoteEditorPath: document.getElementById("remoteEditorPath"),
    remoteEditorText: document.getElementById("remoteEditorText"),
    btnRemoteEditorSave: document.getElementById("btnRemoteEditorSave"),
    btnRemoteEditorHide: document.getElementById("btnRemoteEditorHide"),
    btnRemoteEditorClose: document.getElementById("btnRemoteEditorClose"),

    // Session Manager Modal
    managerModal: document.getElementById("managerModal"),
    sessionList: document.getElementById("sessionList"),
    mgrHost: document.getElementById("mgrHost"),
    mgrPort: document.getElementById("mgrPort"),
    mgrUser: document.getElementById("mgrUser"),
    mgrAuthType: document.getElementById("mgrAuthType"),
    passGroup: document.getElementById("passGroup"),
    mgrPass: document.getElementById("mgrPass"),
    keyGroup: document.getElementById("keyGroup"),
    mgrKeyPath: document.getElementById("mgrKeyPath"),
    mgrKeyPass: document.getElementById("mgrKeyPass"),
    btnMgrSave: document.getElementById("btnMgrSave"),
    btnMgrDelete: document.getElementById("btnMgrDelete"),
    btnMgrConnect: document.getElementById("btnMgrConnect"),
    btnMgrClose: document.getElementById("btnMgrClose"),

    // Master Password Modal
    masterPassModal: document.getElementById("masterPassModal"),
    masterPassInput: document.getElementById("masterPassInput"),
    btnMasterPassSubmit: document.getElementById("btnMasterPassSubmit"),
    btnMasterPassCancel: document.getElementById("btnMasterPassCancel"),

    // Chmod Modal
    chmodModal: document.getElementById("chmodModal"),
    chmodTargetLabel: document.getElementById("chmodTargetLabel"),
    chmodOctalVal: document.getElementById("chmodOctalVal"),
    chkUR: document.getElementById("chkUR"),
    chkUW: document.getElementById("chkUW"),
    chkUX: document.getElementById("chkUX"),
    chkGR: document.getElementById("chkGR"),
    chkGW: document.getElementById("chkGW"),
    chkGX: document.getElementById("chkGX"),
    chkOR: document.getElementById("chkOR"),
    chkOW: document.getElementById("chkOW"),
    chkOX: document.getElementById("chkOX"),
    btnChmodSet777: document.getElementById("btnChmodSet777"),
    btnChmodApply: document.getElementById("btnChmodApply"),
    btnChmodCancel: document.getElementById("btnChmodCancel"),

    // su Password Modal
    suModal: document.getElementById("suModal"),
    suPassInput: document.getElementById("suPassInput"),
    btnSuSubmit: document.getElementById("btnSuSubmit"),
    btnSuCancel: document.getElementById("btnSuCancel")
};

// --- Initialization ---
window.addEventListener("DOMContentLoaded", async () => {
    setupEventListeners();
    setupSplitter();
    await loadLocalDir("");
    await checkMasterPasswordAndLoadProfiles();
});

// Helper for backend calls via Wails go runtime bindings
async function callBackend(methodName, ...args) {
    if (window.go && window.go.main && window.go.main.App && window.go.main.App[methodName]) {
        return await window.go.main.App[methodName](...args);
    } else {
        console.warn(`Backend method ${methodName} not available in mock/test mode.`);
        return null;
    }
}

// Log command action into SSH CLI Terminal Stream
function appendTerminalOutput(text, isCommand = false) {
    if (isCommand) {
        el.terminalOutput.innerText += "\n$ " + text + "\n";
    } else {
        el.terminalOutput.innerText += text;
    }
    el.terminalOutput.scrollTop = el.terminalOutput.scrollHeight;
}

// --- Local File Operations ---
async function loadLocalDir(dirPath) {
    try {
        const res = await callBackend("ListLocalDir", dirPath);
        if (res) {
            currentLocalPath = res.currentPath;
            el.localPathInput.value = currentLocalPath;
            renderFileList(el.localFileTableBody, res.items, "local");
            el.localCount.innerText = `${res.items ? res.items.length : 0} items`;
            selectedLocalItem = null;
        }
    } catch (err) {
        alert("Error loading local directory: " + err);
    }
}

// --- Remote File Operations ---
async function loadRemoteDir(remotePath) {
    try {
        const res = await callBackend("ListRemoteDir", remotePath);
        if (res) {
            currentRemotePath = res.currentPath;
            el.remotePathInput.value = currentRemotePath;
            renderFileList(el.remoteFileTableBody, res.items, "remote");
            el.remoteCount.innerText = `${res.items ? res.items.length : 0} items`;
            selectedRemoteItem = null;
        }
    } catch (err) {
        console.error("Error loading remote directory:", err);
    }
}

// Render Table Rows
function renderFileList(tbody, items, pane) {
    tbody.innerHTML = "";
    if (!items) return;

    items.forEach(item => {
        const tr = document.createElement("tr");
        tr.className = "file-row";

        const icon = item.isDir ? "📁" : "📄";
        const sizeStr = item.isDir ? "<DIR>" : formatBytes(item.size);

        tr.innerHTML = `
            <td><span class="file-icon">${icon}</span>${escapeHtml(item.name)}</td>
            <td>${sizeStr}</td>
            <td style="font-family: monospace;">${item.modeOct || item.mode}</td>
        `;

        tr.addEventListener("click", () => {
            document.querySelectorAll(`.file-row`).forEach(r => r.classList.remove("selected"));
            tr.classList.add("selected");
            activePane = pane;
            if (pane === "local") {
                selectedLocalItem = item;
            } else {
                selectedRemoteItem = item;
            }
        });

        tr.addEventListener("dblclick", async () => {
            if (item.isDir) {
                if (pane === "local") {
                    await loadLocalDir(item.path);
                } else {
                    appendTerminalOutput(`cd ${quotePath(item.path)}`, true);
                    await callBackend("ExecuteRemoteCmd", `cd ${quotePath(item.path)}`);
                    await loadRemoteDir(item.path);
                }
            } else {
                openTextEditor(pane, item);
            }
        });

        // Setup Drag and Drop
        tr.draggable = true;
        tr.addEventListener("dragstart", (e) => {
            e.dataTransfer.setData("application/json", JSON.stringify({ pane, item }));
        });

        tbody.appendChild(tr);
    });

    // Setup Drag and Drop onto Table Container
    const container = pane === "local" ? el.localFileTableBody : el.remoteFileTableBody;
    container.ondragover = (e) => e.preventDefault();
    container.ondrop = async (e) => {
        e.preventDefault();
        const dataStr = e.dataTransfer.getData("application/json");
        if (!dataStr) return;
        const data = JSON.parse(dataStr);

        if (data.pane === "local" && pane === "remote") {
            const remoteDst = currentRemotePath + "/" + data.item.name;
            appendTerminalOutput(`upload ${data.item.path} -> ${remoteDst}`, true);
            await callBackend("UploadFile", data.item.path, remoteDst);
            await loadRemoteDir(currentRemotePath);
        } else if (data.pane === "remote" && pane === "local") {
            const localDst = currentLocalPath + "/" + data.item.name;
            appendTerminalOutput(`download ${data.item.path} -> ${localDst}`, true);
            await callBackend("DownloadFile", data.item.path, localDst);
            await loadLocalDir(currentLocalPath);
        }
    };
}

// Side Pull-out Text Editor Toasts
async function openTextEditor(pane, item) {
    if (pane === "local") {
        try {
            const content = await callBackend("ReadLocalFileText", item.path);
            el.localEditorPath.value = item.path;
            el.localEditorText.value = content;
            el.localEditorToast.classList.add("visible");
            el.localEditorToast.classList.remove("hidden-docked");
        } catch (err) {
            alert("Unable to open local file: " + err);
        }
    } else {
        try {
            const content = await callBackend("ReadRemoteFileText", item.path);
            el.remoteEditorPath.value = item.path;
            el.remoteEditorText.value = content;
            el.remoteEditorToast.classList.add("visible");
            el.remoteEditorToast.classList.remove("hidden-docked");
        } catch (err) {
            alert("Unable to open remote file: " + err);
        }
    }
}

// --- Setup Event Listeners ---
function setupEventListeners() {
    // Navigation & Buttons
    el.btnLocalGo.addEventListener("click", () => loadLocalDir(el.localPathInput.value));
    el.localPathInput.addEventListener("keydown", (e) => { if (e.key === "Enter") loadLocalDir(el.localPathInput.value); });

    el.btnRemoteGo.addEventListener("click", () => {
        appendTerminalOutput(`cd ${quotePath(el.remotePathInput.value)}`, true);
        loadRemoteDir(el.remotePathInput.value);
    });
    el.remotePathInput.addEventListener("keydown", (e) => {
        if (e.key === "Enter") {
            appendTerminalOutput(`cd ${quotePath(el.remotePathInput.value)}`, true);
            loadRemoteDir(el.remotePathInput.value);
        }
    });

    el.btnUp.addEventListener("click", async () => {
        if (activePane === "local") {
            await loadLocalDir(currentLocalPath + "/..");
        } else {
            const upPath = currentRemotePath + "/..";
            appendTerminalOutput(`cd ${upPath}`, true);
            await loadRemoteDir(upPath);
        }
    });

    // Clipboard: Cut / Copy / Paste
    el.btnCut.addEventListener("click", () => {
        const item = activePane === "local" ? selectedLocalItem : selectedRemoteItem;
        if (!item) {
            alert("Select an item to cut.");
            return;
        }
        clipboard = { action: "cut", pane: activePane, item };
        alert(`Cut '${item.name}' to clipboard.`);
    });

    el.btnCopy.addEventListener("click", () => {
        const item = activePane === "local" ? selectedLocalItem : selectedRemoteItem;
        if (!item) {
            alert("Select an item to copy.");
            return;
        }
        clipboard = { action: "copy", pane: activePane, item };
        alert(`Copied '${item.name}' to clipboard.`);
    });

    el.btnPaste.addEventListener("click", async () => {
        if (!clipboard || !clipboard.item) {
            alert("Clipboard is empty.");
            return;
        }

        const srcItem = clipboard.item;
        const srcPane = clipboard.pane;
        const targetPane = activePane;

        if (srcPane === "local" && targetPane === "local") {
            const dstPath = currentLocalPath + "/" + srcItem.name;
            if (clipboard.action === "cut") {
                await callBackend("RenameLocal", srcItem.path, dstPath);
            } else {
                await callBackend("CopyLocalFile", srcItem.path, dstPath);
            }
            await loadLocalDir(currentLocalPath);
        } else if (srcPane === "remote" && targetPane === "remote") {
            const dstPath = currentRemotePath + "/" + srcItem.name;
            if (clipboard.action === "cut") {
                appendTerminalOutput(`mv ${quotePath(srcItem.path)} ${quotePath(dstPath)}`, true);
                await callBackend("RenameRemote", srcItem.path, dstPath);
            } else {
                appendTerminalOutput(`cp -r ${quotePath(srcItem.path)} ${quotePath(dstPath)}`, true);
                await callBackend("CopyRemoteFile", srcItem.path, dstPath);
            }
            await loadRemoteDir(currentRemotePath);
        } else if (srcPane === "local" && targetPane === "remote") {
            const dstPath = currentRemotePath + "/" + srcItem.name;
            appendTerminalOutput(`upload ${srcItem.path} -> ${dstPath}`, true);
            await callBackend("UploadFile", srcItem.path, dstPath);
            if (clipboard.action === "cut") {
                await callBackend("DeleteLocal", srcItem.path);
                await loadLocalDir(currentLocalPath);
            }
            await loadRemoteDir(currentRemotePath);
        } else if (srcPane === "remote" && targetPane === "local") {
            const dstPath = currentLocalPath + "/" + srcItem.name;
            appendTerminalOutput(`download ${srcItem.path} -> ${dstPath}`, true);
            await callBackend("DownloadFile", srcItem.path, dstPath);
            if (clipboard.action === "cut") {
                await callBackend("DeleteRemote", srcItem.path);
                await loadRemoteDir(currentRemotePath);
            }
            await loadLocalDir(currentLocalPath);
        }

        if (clipboard.action === "cut") {
            clipboard = null;
        }
    });

    el.btnRefresh.addEventListener("click", () => {
        loadLocalDir(currentLocalPath);
        loadRemoteDir(currentRemotePath);
    });

    el.btnNewFolder.addEventListener("click", async () => {
        const folderName = prompt("Enter new folder name:");
        if (!folderName) return;

        if (activePane === "local") {
            const path = currentLocalPath + "/" + folderName;
            await callBackend("CreateLocalFolder", path);
            await loadLocalDir(currentLocalPath);
        } else {
            const path = currentRemotePath + "/" + folderName;
            appendTerminalOutput(`mkdir ${quotePath(path)}`, true);
            await callBackend("CreateRemoteFolder", path);
            await loadRemoteDir(currentRemotePath);
        }
    });

    el.btnNewText.addEventListener("click", async () => {
        const fileName = prompt("Enter new file name:", "newfile.txt");
        if (!fileName) return;

        if (activePane === "local") {
            const filePath = currentLocalPath + "/" + fileName;
            await callBackend("CreateLocalFile", filePath);
            await loadLocalDir(currentLocalPath);
            openTextEditor("local", { path: filePath, name: fileName });
        } else {
            const filePath = currentRemotePath + "/" + fileName;
            appendTerminalOutput(`touch ${quotePath(filePath)}`, true);
            await callBackend("CreateRemoteFile", filePath);
            await loadRemoteDir(currentRemotePath);
            openTextEditor("remote", { path: filePath, name: fileName });
        }
    });

    el.btnDelete.addEventListener("click", async () => {
        const item = activePane === "local" ? selectedLocalItem : selectedRemoteItem;
        if (!item) {
            alert("Please select an item to delete.");
            return;
        }
        if (!confirm(`Are you sure you want to delete ${item.name}?`)) return;

        if (activePane === "local") {
            await callBackend("DeleteLocal", item.path);
            await loadLocalDir(currentLocalPath);
        } else {
            appendTerminalOutput(`rm -rf ${quotePath(item.path)}`, true);
            await callBackend("DeleteRemote", item.path);
            await loadRemoteDir(currentRemotePath);
        }
    });

    el.btnRename.addEventListener("click", async () => {
        const item = activePane === "local" ? selectedLocalItem : selectedRemoteItem;
        if (!item) {
            alert("Please select an item to rename.");
            return;
        }
        const newName = prompt("Enter new name:", item.name);
        if (!newName || newName === item.name) return;

        if (activePane === "local") {
            const newPath = currentLocalPath + "/" + newName;
            await callBackend("RenameLocal", item.path, newPath);
            await loadLocalDir(currentLocalPath);
        } else {
            const newPath = currentRemotePath + "/" + newName;
            appendTerminalOutput(`mv ${quotePath(item.path)} ${quotePath(newPath)}`, true);
            await callBackend("RenameRemote", item.path, newPath);
            await loadRemoteDir(currentRemotePath);
        }
    });

    // Chmod Overlay Modal
    el.btnChmod.addEventListener("click", () => {
        const item = activePane === "local" ? selectedLocalItem : selectedRemoteItem;
        if (!item) {
            alert("Please select an item for Chmod.");
            return;
        }
        el.chmodTargetLabel.innerText = "File: " + item.path;
        parseAndSetChmodCheckboxes(item.modeOct || "0755");
        el.chmodModal.classList.remove("hidden");
    });

    el.btnChmodSet777.addEventListener("click", () => {
        setChmodCheckboxes(true, true, true, true, true, true, true, true, true);
        updateChmodOctalLabel();
    });

    const checkboxes = [el.chkUR, el.chkUW, el.chkUX, el.chkGR, el.chkGW, el.chkGX, el.chkOR, el.chkOW, el.chkOX];
    checkboxes.forEach(chk => chk.addEventListener("change", updateChmodOctalLabel));

    el.btnChmodCancel.addEventListener("click", () => el.chmodModal.classList.add("hidden"));
    el.btnChmodApply.addEventListener("click", async () => {
        const item = activePane === "local" ? selectedLocalItem : selectedRemoteItem;
        const oct = el.chmodOctalVal.innerText;
        if (activePane === "local") {
            await callBackend("ChmodLocal", item.path, oct);
            await loadLocalDir(currentLocalPath);
        } else {
            appendTerminalOutput(`chmod ${oct} ${quotePath(item.path)}`, true);
            await callBackend("ChmodRemote", item.path, oct);
            await loadRemoteDir(currentRemotePath);
        }
        el.chmodModal.classList.add("hidden");
    });

    // Local / Remote Editor Toast Actions
    el.btnLocalEditorSave.addEventListener("click", async () => {
        await callBackend("WriteLocalFileText", el.localEditorPath.value, el.localEditorText.value);
        alert("Local file saved successfully.");
        await loadLocalDir(currentLocalPath);
    });
    el.btnLocalEditorHide.addEventListener("click", () => el.localEditorToast.classList.toggle("hidden-docked"));
    el.btnLocalEditorClose.addEventListener("click", () => el.localEditorToast.classList.remove("visible"));

    el.btnRemoteEditorSave.addEventListener("click", async () => {
        appendTerminalOutput(`save file ${el.remoteEditorPath.value}`, true);
        await callBackend("WriteRemoteFileText", el.remoteEditorPath.value, el.remoteEditorText.value);
        alert("Remote file saved successfully.");
        await loadRemoteDir(currentRemotePath);
    });
    el.btnRemoteEditorHide.addEventListener("click", () => el.remoteEditorToast.classList.toggle("hidden-docked"));
    el.btnRemoteEditorClose.addEventListener("click", () => el.remoteEditorToast.classList.remove("visible"));

    // Session Manager Modal & Actions
    el.btnManager.addEventListener("click", () => openSessionManager());
    el.btnMgrClose.addEventListener("click", () => el.managerModal.classList.add("hidden"));

    el.mgrAuthType.addEventListener("change", () => {
        if (el.mgrAuthType.value === "key") {
            el.passGroup.classList.add("hidden");
            el.keyGroup.classList.remove("hidden");
        } else {
            el.passGroup.classList.remove("hidden");
            el.keyGroup.classList.add("hidden");
        }
    });

    el.btnMgrSave.addEventListener("click", async () => {
        const prof = {
            id: selectedProfileId || `${el.mgrUser.value}_${el.mgrPort.value}_${el.mgrHost.value}`,
            name: `${el.mgrUser.value}@${el.mgrHost.value}`,
            host: el.mgrHost.value,
            port: parseInt(el.mgrPort.value, 10) || 22,
            username: el.mgrUser.value,
            authType: el.mgrAuthType.value,
            password: el.mgrPass.value,
            keyPath: el.mgrKeyPath.value,
            keyPassphrase: el.mgrKeyPass.value
        };
        await callBackend("SaveProfile", prof);
        await reloadProfiles();
    });

    el.btnMgrDelete.addEventListener("click", async () => {
        if (selectedProfileId) {
            await callBackend("DeleteProfile", selectedProfileId);
            selectedProfileId = null;
            await reloadProfiles();
        }
    });

    el.btnMgrConnect.addEventListener("click", async () => {
        const host = el.mgrHost.value;
        const port = parseInt(el.mgrPort.value, 10) || 22;
        const user = el.mgrUser.value;
        const authType = el.mgrAuthType.value;
        const pass = el.mgrPass.value;
        const keyPath = el.mgrKeyPath.value;
        const keyPass = el.mgrKeyPass.value;

        el.connStatus.innerText = "Connecting...";
        try {
            await callBackend("ConnectSSH", host, port, user, authType, pass, keyPath, keyPass);
            el.connStatus.innerText = `Connected: ${user}@${host}:${port}`;
            el.connStatus.style.color = "#a6e3a1";
            el.managerModal.classList.add("hidden");
            startTerminalPolling();
            await loadRemoteDir("~");
        } catch (err) {
            el.connStatus.innerText = "Connection Failed";
            el.connStatus.style.color = "#f38ba8";
            alert("SSH Connection Error: " + err);
        }
    });

    el.btnDisconnect.addEventListener("click", () => {
        callBackend("DisconnectSSH");
        stopTerminalPolling();
        el.connStatus.innerText = "Disconnected";
        el.connStatus.style.color = "#a6adc8";
        el.remoteFileTableBody.innerHTML = "";
    });

    // SSH Terminal CLI Input
    el.terminalInput.addEventListener("keydown", async (e) => {
        if (e.key === "Enter") {
            const input = el.terminalInput.value.trim();
            el.terminalInput.value = "";
            if (!input) return;

            if (input === "exit") {
                await callBackend("SetSSHRootMode", false);
                el.terminalPrompt.innerText = "SSH>";
            }

            if (input.startsWith("cdl ")) {
                const targetLocal = input.substring(4).trim();
                appendTerminalOutput(`cdl ${targetLocal}`, true);
                await loadLocalDir(currentLocalPath + "/" + targetLocal);
                return;
            }

            if (input === "su" || input.startsWith("su ")) {
                appendTerminalOutput(`su`, true);
                el.suPassInput.value = "";
                el.suModal.classList.remove("hidden");
                return;
            }

            appendTerminalOutput(input, true);
            await callBackend("SendTerminalInput", input + "\n");

            if (input.startsWith("cd ") || input === "cd") {
                const target = input.substring(3).trim() || "~";
                setTimeout(async () => {
                    await loadRemoteDir(target);
                }, 300);
            }
        }
    });

    // su Modal Actions
    el.btnSuCancel.addEventListener("click", () => el.suModal.classList.add("hidden"));
    el.btnSuSubmit.addEventListener("click", async () => {
        const pass = el.suPassInput.value;
        el.suModal.classList.add("hidden");
        await callBackend("SendTerminalInput", pass + "\n");
        await callBackend("SetSSHRootMode", true);
        el.terminalPrompt.innerText = "root@SSH#";
        setTimeout(async () => {
            await loadRemoteDir(currentRemotePath);
        }, 500);
    });

    // Master Password Modal Actions
    el.btnMasterPassCancel.addEventListener("click", () => el.masterPassModal.classList.add("hidden"));
    el.btnMasterPassSubmit.addEventListener("click", async () => {
        const pass = el.masterPassInput.value;
        const hasMaster = await callBackend("HasMasterPassword");
        if (!hasMaster) {
            await callBackend("SetMasterPassword", pass);
            el.masterPassModal.classList.add("hidden");
            await reloadProfiles();
        } else {
            const ok = await callBackend("VerifyMasterPassword", pass);
            if (ok) {
                el.masterPassModal.classList.add("hidden");
                await reloadProfiles();
            } else {
                alert("Incorrect Master Password");
            }
        }
    });
}

// Master Password & Profile Manager Logic
async function checkMasterPasswordAndLoadProfiles() {
    const hasMaster = await callBackend("HasMasterPassword");
    if (hasMaster) {
        el.masterPassModal.classList.remove("hidden");
    } else {
        await reloadProfiles();
    }
}

async function openSessionManager() {
    el.managerModal.classList.remove("hidden");
    await reloadProfiles();
}

async function reloadProfiles() {
    savedProfiles = (await callBackend("GetProfiles")) || [];
    el.sessionList.innerHTML = "";

    savedProfiles.forEach(p => {
        const div = document.createElement("div");
        div.className = "session-item" + (p.id === selectedProfileId ? " selected" : "");
        div.innerHTML = `<span>${p.name}</span><span style="color:#a6adc8;">${p.authType}</span>`;
        div.addEventListener("click", () => {
            selectedProfileId = p.id;
            el.mgrHost.value = p.host;
            el.mgrPort.value = p.port;
            el.mgrUser.value = p.username;
            el.mgrAuthType.value = p.authType;
            el.mgrPass.value = p.password || "";
            el.mgrKeyPath.value = p.keyPath || "";
            el.mgrKeyPass.value = p.keyPassphrase || "";
            el.mgrAuthType.dispatchEvent(new Event("change"));
            reloadProfiles();
        });
        el.sessionList.appendChild(div);
    });
}

// Resizable Splitter Logic
function setupSplitter() {
    let isDragging = false;
    let startY = 0;
    let startPanesHeight = 0;

    el.splitterH.addEventListener("mousedown", (e) => {
        isDragging = true;
        startY = e.clientY;
        startPanesHeight = el.filePanesContainer.offsetHeight;
        el.splitterH.classList.add("dragging");
    });

    document.addEventListener("mousemove", (e) => {
        if (!isDragging) return;
        const deltaY = e.clientY - startY;
        const newHeight = startPanesHeight + deltaY;
        const containerHeight = el.filePanesContainer.parentElement.offsetHeight;
        if (newHeight > 100 && newHeight < containerHeight - 100) {
            el.filePanesContainer.style.height = `${newHeight}px`;
        }
    });

    document.addEventListener("mouseup", () => {
        if (isDragging) {
            isDragging = false;
            el.splitterH.classList.remove("dragging");
        }
    });
}

// Terminal Output Polling
function startTerminalPolling() {
    stopTerminalPolling();
    terminalPollingInterval = setInterval(async () => {
        const out = await callBackend("PollTerminalOutput");
        if (out) {
            appendTerminalOutput(out);
        }
    }, 150);
}

function stopTerminalPolling() {
    if (terminalPollingInterval) {
        clearInterval(terminalPollingInterval);
        terminalPollingInterval = null;
    }
}

// Helper Functions
function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function escapeHtml(str) {
    return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function quotePath(p) {
    return "'" + p.replace(/'/g, "'\\''") + "'";
}

function parseAndSetChmodCheckboxes(octalStr) {
    let val = parseInt(octalStr, 8) || 0755;
    const u = (val >> 6) & 7;
    const g = (val >> 3) & 7;
    const o = val & 7;

    setChmodCheckboxes(
        Boolean(u & 4), Boolean(u & 2), Boolean(u & 1),
        Boolean(g & 4), Boolean(g & 2), Boolean(g & 1),
        Boolean(o & 4), Boolean(o & 2), Boolean(o & 1)
    );
    updateChmodOctalLabel();
}

function setChmodCheckboxes(ur, uw, ux, gr, gw, gx, or, ow, ox) {
    el.chkUR.checked = ur; el.chkUW.checked = uw; el.chkUX.checked = ux;
    el.chkGR.checked = gr; el.chkGW.checked = gw; el.chkGX.checked = gx;
    el.chkOR.checked = or; el.chkOW.checked = ow; el.chkOX.checked = ox;
}

function updateChmodOctalLabel() {
    const u = (el.chkUR.checked ? 4 : 0) + (el.chkUW.checked ? 2 : 0) + (el.chkUX.checked ? 1 : 0);
    const g = (el.chkGR.checked ? 4 : 0) + (el.chkGW.checked ? 2 : 0) + (el.chkGX.checked ? 1 : 0);
    const o = (el.chkOR.checked ? 4 : 0) + (el.chkOW.checked ? 2 : 0) + (el.chkOX.checked ? 1 : 0);
    el.chmodOctalVal.innerText = `0${u}${g}${o}`;
}

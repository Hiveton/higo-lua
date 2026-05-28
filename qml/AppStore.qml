import QtQuick
import QtCore

Item {
    id: store

    property string machineState: "运行中"
    property string currentProgram: "HVT-MAIN-8H-001"
    property int boardCount: 1286
    property int cph: 48200
    property real cycleTime: 18.6
    property real yieldRate: 99.2
    property int selectedFeeder: 0
    property int selectedProgramRow: 0
    property int selectedAlarm: 0
    property int selectedHead: 0
    property int selectedCamera: 0
    property int selectedRail: 0
    property int selectedTray: 0
    property int selectedMaintenance: 0

    property var feeders: []
    property var programRows: []
    property var heads: []
    property var cameras: []
    property var rails: []
    property var trays: []
    property var alarms: []
    property var maintenance: []
    property bool isLoading: false

    signal changed()

    Settings {
        id: persistent
        category: "hmi-data"
        property string snapshotJson: ""
    }

    onChanged: {
        if (!isLoading)
            persistSnapshot()
    }

    Component.onCompleted: {
        if (!loadPersistedSnapshot())
            resetDemoData()
    }

    function createSnapshot() {
        return {
            version: 1,
            machineState: machineState,
            currentProgram: currentProgram,
            boardCount: boardCount,
            cph: cph,
            cycleTime: cycleTime,
            yieldRate: yieldRate,
            selectedFeeder: selectedFeeder,
            selectedProgramRow: selectedProgramRow,
            selectedAlarm: selectedAlarm,
            selectedHead: selectedHead,
            selectedCamera: selectedCamera,
            selectedRail: selectedRail,
            selectedTray: selectedTray,
            selectedMaintenance: selectedMaintenance,
            feeders: cloneArray(feeders),
            programRows: cloneArray(programRows),
            heads: cloneArray(heads),
            cameras: cloneArray(cameras),
            rails: cloneArray(rails),
            trays: cloneArray(trays),
            alarms: cloneArray(alarms),
            maintenance: cloneArray(maintenance)
        }
    }

    function applySnapshot(snapshot) {
        if (!snapshot) return false
        isLoading = true
        machineState = snapshot.machineState || "运行中"
        currentProgram = snapshot.currentProgram || "HVT-MAIN-8H-001"
        boardCount = snapshot.boardCount || 0
        cph = snapshot.cph || 0
        cycleTime = snapshot.cycleTime || 0
        yieldRate = snapshot.yieldRate || 0
        selectedFeeder = snapshot.selectedFeeder || 0
        selectedProgramRow = snapshot.selectedProgramRow || 0
        selectedAlarm = snapshot.selectedAlarm || 0
        selectedHead = snapshot.selectedHead || 0
        selectedCamera = snapshot.selectedCamera || 0
        selectedRail = snapshot.selectedRail || 0
        selectedTray = snapshot.selectedTray || 0
        selectedMaintenance = snapshot.selectedMaintenance || 0
        feeders = snapshot.feeders || []
        programRows = snapshot.programRows || []
        heads = snapshot.heads || []
        cameras = snapshot.cameras || []
        rails = snapshot.rails || []
        trays = snapshot.trays || []
        alarms = snapshot.alarms || []
        maintenance = snapshot.maintenance || []
        isLoading = false
        changed()
        return true
    }

    function persistSnapshot() {
        persistent.snapshotJson = JSON.stringify(createSnapshot())
    }

    function loadPersistedSnapshot() {
        if (!persistent.snapshotJson || persistent.snapshotJson.length === 0)
            return false
        try {
            return applySnapshot(JSON.parse(persistent.snapshotJson))
        } catch (e) {
            persistent.snapshotJson = ""
            return false
        }
    }

    function clearPersistedSnapshot() {
        persistent.snapshotJson = ""
        resetDemoData()
    }

    function exportSnapshotJson() {
        return JSON.stringify(createSnapshot(), null, 2)
    }

    function importSnapshotJson(jsonText) {
        try {
            var snapshot = JSON.parse(jsonText)
            var ok = applySnapshot(snapshot)
            if (ok)
                persistSnapshot()
            return ok
        } catch (e) {
            addAlarm({ time: "现在", level: "严重", code: "IMPORT", module: "系统", desc: "数据导入失败", action: String(e), status: "未确认" })
            return false
        }
    }

    function runCrudSelfTest() {
        var original = createSnapshot()
        var ok = true
        try {
            resetDemoData()

            var feederCount = feeders.length
            addFeeder({ slot: "T01", face: "前", material: "SELFTEST_R", packageName: "0603", pitch: "4", qty: "100", head: "H1", nozzle: "CN040", status: "正常" })
            ok = ok && feeders.length === feederCount + 1 && feeders[0].material === "SELFTEST_R"
            updateFeeder(0, { slot: "T01", face: "前", material: "SELFTEST_R_EDIT", packageName: "0603", pitch: "4", qty: "99", head: "H2", nozzle: "CN065", status: "待校验" })
            ok = ok && feeders[0].material === "SELFTEST_R_EDIT" && feeders[0].head === "H2"
            deleteFeeder(0)
            ok = ok && feeders.length === feederCount

            var programCount = programRows.length
            addProgramRow({ ref: "ST1", material: "SELFTEST_IC", source: "托盘-01", head: "H3", nozzle: "CN100", x: "1.000", y: "2.000", angle: "90", strategy: "大物件相机", status: "待校验" })
            ok = ok && programRows.length === programCount + 1
            updateProgramRow(programRows.length - 1, { ref: "ST1", material: "SELFTEST_IC_EDIT", source: "托盘-02", head: "H4", nozzle: "CN100", x: "3.000", y: "4.000", angle: "180", strategy: "Mark相机", status: "已校验" })
            ok = ok && programRows[programRows.length - 1].material === "SELFTEST_IC_EDIT"
            deleteProgramRow(programRows.length - 1)
            ok = ok && programRows.length === programCount

            upsertHead(0, { id: "H1", nozzle: "SELFTEST_N", status: "正常", vacuum: "-80", z: "12.00", theta: "0", success: "99.9%" })
            ok = ok && heads[0].nozzle === "SELFTEST_N"
            upsertCamera(0, { name: "飞行相机", exposure: "1111", gain: "1.0", light: "同轴", mode: "元件识别", offset: "0/0", status: "OK" })
            ok = ok && cameras[0].exposure === "1111"
            upsertRail(0, { area: "进板段", board: "SELFTEST_BOARD", width: "100.0", speed: "200", sensor: "有板", status: "正常" })
            ok = ok && rails[0].board === "SELFTEST_BOARD"
            upsertTray(0, { tray: "Tray-ST", material: "SELFTEST_TRAY", pockets: 1, used: 0, status: "正常" })
            ok = ok && trays[0].tray === "Tray-ST"
            upsertMaintenance(0, { item: "SELFTEST_MAINT", module: "系统", interval: "1天", next: "明日", status: "正常" })
            ok = ok && maintenance[0].item === "SELFTEST_MAINT"
            addAlarm({ time: "现在", level: "提示", code: "SELFTEST", module: "系统", desc: "自检", action: "无", status: "未确认" })
            ok = ok && alarms[0].code === "SELFTEST"
            clearAlarm(0)
            ok = ok && alarms[0].status === "已确认"

            var exported = exportSnapshotJson()
            ok = ok && exported.indexOf("SELFTEST_BOARD") >= 0
            clearPersistedSnapshot()
            ok = ok && importSnapshotJson(exported)
            ok = ok && rails[0].board === "SELFTEST_BOARD" && alarms[0].code === "SELFTEST"
            persistSnapshot()
            ok = ok && loadPersistedSnapshot() && rails[0].board === "SELFTEST_BOARD"
        } catch (e) {
            console.error("CRUD self-test exception:", e)
            ok = false
        }
        applySnapshot(original)
        persistSnapshot()
        return ok
    }

    function resetDemoData() {
        isLoading = true
        var f = []
        for (var i = 1; i <= 120; i++) {
            var face = i <= 60 ? "前" : "后"
            var slot = i <= 60 ? i : i - 60
            f.push({
                slot: (slot < 10 ? "0" : "") + slot,
                face: face,
                material: ["R0603-10K", "C0402-100N", "LED0603-G", "QFN32-MCU", "SOT23-MOS"][i % 5],
                packageName: ["0603", "0402", "0603", "QFN32", "SOT23"][i % 5],
                pitch: [2, 4, 2, 8, 4][i % 5],
                qty: 6000 - i * 23,
                head: "H" + ((i % 8) + 1),
                nozzle: ["CN030", "CN040", "CN065", "CN100"][i % 4],
                status: i % 17 === 0 ? "缺料" : (i % 11 === 0 ? "待校验" : "正常")
            })
        }
        feeders = f

        programRows = [
            { ref: "R101", material: "R0603-10K", source: "前-01", head: "H1", nozzle: "CN040", x: "42.120", y: "18.340", angle: "90", strategy: "飞行相机", status: "已校验" },
            { ref: "C203", material: "C0402-100N", source: "前-12", head: "H2", nozzle: "CN030", x: "48.600", y: "21.000", angle: "0", strategy: "飞行相机", status: "已校验" },
            { ref: "U3", material: "QFN32-MCU", source: "托盘-03", head: "H5", nozzle: "CN100", x: "63.250", y: "44.820", angle: "270", strategy: "大物件相机", status: "待确认" },
            { ref: "FID1", material: "Mark", source: "PCB", head: "-", nozzle: "-", x: "8.000", y: "8.000", angle: "0", strategy: "Mark相机", status: "基准点" }
        ]

        heads = []
        for (var h = 1; h <= 8; h++) {
            heads.push({
                id: "H" + h,
                nozzle: ["CN030", "CN040", "CN065", "CN100"][h % 4],
                status: h === 7 ? "待保养" : "正常",
                vacuum: -72 + h,
                z: (12.2 + h / 10).toFixed(2),
                theta: (h * 45) % 360,
                success: (99.5 - h / 10).toFixed(1) + "%"
            })
        }

        cameras = [
            { name: "飞行相机", exposure: 2400, gain: 1.2, light: "同轴 70%", mode: "元件识别", offset: "0.012 / -0.006", status: "OK" },
            { name: "Mark相机", exposure: 1800, gain: 1.0, light: "环形 55%", mode: "Mark定位", offset: "-0.004 / 0.003", status: "OK" },
            { name: "大物件相机", exposure: 3200, gain: 1.4, light: "低角度 80%", mode: "大物件识别", offset: "0.020 / 0.010", status: "OK" }
        ]

        rails = [
            { area: "进板段", board: "PCB-1286", width: "142.0", speed: "280", sensor: "有板", status: "等待贴装" },
            { area: "贴装段", board: "PCB-1287", width: "142.0", speed: "120", sensor: "夹紧", status: "贴装中" },
            { area: "出板段", board: "PCB-1285", width: "142.0", speed: "300", sensor: "有板", status: "出板中" }
        ]

        trays = [
            { tray: "Tray-01", material: "QFN32-MCU", pockets: 48, used: 13, status: "正常" },
            { tray: "Tray-02", material: "CONN-BTB", pockets: 32, used: 8, status: "正常" },
            { tray: "Tray-03", material: "SHIELD-L", pockets: 24, used: 3, status: "待校准" }
        ]

        alarms = [
            { time: "10:22:18", level: "警告", code: "FDR-017", module: "飞达", desc: "前17飞达余料不足", action: "更换料带并扫码", status: "未确认" },
            { time: "10:31:02", level: "严重", code: "VIS-004", module: "视觉", desc: "Mark识别失败", action: "清洁基准点并重新识别", status: "处理中" },
            { time: "10:40:51", level: "提示", code: "NZL-008", module: "贴头", desc: "H8吸嘴寿命接近阈值", action: "排产间隙更换吸嘴", status: "已确认" }
        ]

        maintenance = [
            { item: "吸嘴清洁", module: "贴头系统", interval: "8小时", next: "今日 18:00", status: "待执行" },
            { item: "飞达保养", module: "飞达系统", interval: "7天", next: "05-20", status: "正常" },
            { item: "相机标定", module: "视觉系统", interval: "30天", next: "06-01", status: "正常" },
            { item: "轨道润滑", module: "轨道系统", interval: "14天", next: "05-24", status: "正常" }
        ]
        isLoading = false
        changed()
    }

    function cloneArray(arr) {
        return JSON.parse(JSON.stringify(arr))
    }

    function addFeeder(row) {
        var next = cloneArray(feeders)
        next.unshift(row)
        feeders = next
        changed()
    }

    function updateFeeder(index, row) {
        if (index < 0 || index >= feeders.length) return
        var next = cloneArray(feeders)
        next[index] = row
        feeders = next
        changed()
    }

    function deleteFeeder(index) {
        if (index < 0 || index >= feeders.length) return
        var next = cloneArray(feeders)
        next.splice(index, 1)
        feeders = next
        selectedFeeder = Math.max(0, Math.min(selectedFeeder, next.length - 1))
        changed()
    }

    function addProgramRow(row) {
        var next = cloneArray(programRows)
        next.push(row)
        programRows = next
        changed()
    }

    function updateProgramRow(index, row) {
        if (index < 0 || index >= programRows.length) return
        var next = cloneArray(programRows)
        next[index] = row
        programRows = next
        changed()
    }

    function deleteProgramRow(index) {
        if (index < 0 || index >= programRows.length) return
        var next = cloneArray(programRows)
        next.splice(index, 1)
        programRows = next
        selectedProgramRow = Math.max(0, Math.min(selectedProgramRow, next.length - 1))
        changed()
    }

    function upsertHead(index, row) {
        var next = cloneArray(heads)
        if (index < 0 || index >= next.length) next.push(row); else next[index] = row
        heads = next
        selectedHead = Math.max(0, Math.min(index < 0 ? next.length - 1 : index, next.length - 1))
        changed()
    }

    function deleteHead(index) {
        if (index < 0 || index >= heads.length) return
        var next = cloneArray(heads)
        next.splice(index, 1)
        heads = next
        selectedHead = Math.max(0, Math.min(selectedHead, next.length - 1))
        changed()
    }

    function upsertCamera(index, row) {
        var next = cloneArray(cameras)
        if (index < 0 || index >= next.length) next.push(row); else next[index] = row
        cameras = next
        selectedCamera = Math.max(0, Math.min(index < 0 ? next.length - 1 : index, next.length - 1))
        changed()
    }

    function deleteCamera(index) {
        if (index < 0 || index >= cameras.length) return
        var next = cloneArray(cameras)
        next.splice(index, 1)
        cameras = next
        selectedCamera = Math.max(0, Math.min(selectedCamera, next.length - 1))
        changed()
    }

    function upsertRail(index, row) {
        var next = cloneArray(rails)
        if (index < 0 || index >= next.length) next.push(row); else next[index] = row
        rails = next
        selectedRail = Math.max(0, Math.min(index < 0 ? next.length - 1 : index, next.length - 1))
        changed()
    }

    function deleteRail(index) {
        if (index < 0 || index >= rails.length) return
        var next = cloneArray(rails)
        next.splice(index, 1)
        rails = next
        selectedRail = Math.max(0, Math.min(selectedRail, next.length - 1))
        changed()
    }

    function upsertTray(index, row) {
        var next = cloneArray(trays)
        if (index < 0 || index >= next.length) next.push(row); else next[index] = row
        trays = next
        selectedTray = Math.max(0, Math.min(index < 0 ? next.length - 1 : index, next.length - 1))
        changed()
    }

    function deleteTray(index) {
        if (index < 0 || index >= trays.length) return
        var next = cloneArray(trays)
        next.splice(index, 1)
        trays = next
        selectedTray = Math.max(0, Math.min(selectedTray, next.length - 1))
        changed()
    }

    function upsertMaintenance(index, row) {
        var next = cloneArray(maintenance)
        if (index < 0 || index >= next.length) next.push(row); else next[index] = row
        maintenance = next
        selectedMaintenance = Math.max(0, Math.min(index < 0 ? next.length - 1 : index, next.length - 1))
        changed()
    }

    function deleteMaintenance(index) {
        if (index < 0 || index >= maintenance.length) return
        var next = cloneArray(maintenance)
        next.splice(index, 1)
        maintenance = next
        selectedMaintenance = Math.max(0, Math.min(selectedMaintenance, next.length - 1))
        changed()
    }

    function addAlarm(row) {
        var next = cloneArray(alarms)
        next.unshift(row)
        alarms = next
        changed()
    }

    function clearAlarm(index) {
        if (index < 0 || index >= alarms.length) return
        var next = cloneArray(alarms)
        next[index].status = "已确认"
        alarms = next
        changed()
    }
}

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    property var store
    property int pageIndex: 0
    property string source: ""
    property string pageName: ""
    property real designWidth: 1672
    property real designHeight: 941
    property string editMode: "create"
    property var activeFields: []
    property var activeRecord: ({})

    signal navigate(int index)

    readonly property var productionFields: [
        { label: "当前程序", key: "currentProgram", value: "DEMO_BOARD_A" },
        { label: "生产计数", key: "boardCount", value: "12345 / 20000" },
        { label: "贴装速度", key: "cph", value: "48500 CPH" },
        { label: "节拍时间", key: "cycleTime", value: "1.24 s" },
        { label: "良率", key: "yieldRate", value: "99.56%" },
        { label: "设备状态", key: "machineState", value: "在线" }
    ]
    readonly property var programFields: [
        { label: "位号", key: "ref", value: "R101" },
        { label: "物料", key: "material", value: "R0603-10K" },
        { label: "飞达槽位", key: "source", value: "前-023" },
        { label: "贴头编号", key: "head", value: "H1" },
        { label: "吸嘴型号", key: "nozzle", value: "CN040" },
        { label: "X坐标", key: "x", value: "126.350" },
        { label: "Y坐标", key: "y", value: "78.420" },
        { label: "角度", key: "angle", value: "180.0" },
        { label: "视觉策略", key: "strategy", value: "飞行相机" }
    ]
    readonly property var feederFields: [
        { label: "前60飞达", key: "face", value: "前" },
        { label: "后60飞达", key: "rearFace", value: "后" },
        { label: "飞达槽位", key: "slot", value: "023" },
        { label: "物料", key: "material", value: "R0603-10K" },
        { label: "封装", key: "packageName", value: "0603" },
        { label: "间距", key: "pitch", value: "4" },
        { label: "剩余数量", key: "qty", value: "1234" },
        { label: "贴头编号", key: "head", value: "H1" },
        { label: "吸嘴型号", key: "nozzle", value: "CN040" }
    ]
    readonly property var headFields: [
        { label: "贴头编号", key: "id", value: "H1" },
        { label: "吸嘴型号", key: "nozzle", value: "CN040" },
        { label: "真空值", key: "vacuum", value: "-82.1" },
        { label: "Z轴", key: "z", value: "12.20" },
        { label: "旋转角", key: "theta", value: "180" },
        { label: "成功率", key: "success", value: "99.5%" },
        { label: "状态", key: "status", value: "正常" }
    ]
    readonly property var visionFields: [
        { label: "飞行相机", key: "name", value: "飞行相机" },
        { label: "Mark相机", key: "markCamera", value: "Mark相机" },
        { label: "大物件相机", key: "largeCamera", value: "大物件相机" },
        { label: "曝光", key: "exposure", value: "2400" },
        { label: "增益", key: "gain", value: "1.2" },
        { label: "光源", key: "light", value: "同轴 70%" },
        { label: "视觉模式", key: "mode", value: "元件识别" },
        { label: "偏移", key: "offset", value: "0.012 / -0.006" }
    ]
    readonly property var conveyorFields: [
        { label: "进板段", key: "area", value: "进板段" },
        { label: "贴装段", key: "mountArea", value: "贴装段" },
        { label: "出板段", key: "outArea", value: "出板段" },
        { label: "轨道宽度", key: "width", value: "142.0" },
        { label: "速度", key: "speed", value: "280" },
        { label: "托盘编号", key: "tray", value: "Tray-01" },
        { label: "PCB编号", key: "board", value: "PCB-1287" },
        { label: "状态", key: "status", value: "贴装中" }
    ]
    readonly property var motionFields: [
        { label: "X轴", key: "xAxis", value: "128.420" },
        { label: "Y轴", key: "yAxis", value: "64.120" },
        { label: "Z轴", key: "zAxis", value: "12.20" },
        { label: "R轴", key: "rAxis", value: "180.0" },
        { label: "轨道宽度", key: "railWidth", value: "142.0" },
        { label: "速度", key: "speed", value: "35%" },
        { label: "伺服状态", key: "servo", value: "使能" }
    ]
    readonly property var maintenanceFields: [
        { label: "维护项目", key: "item", value: "吸嘴清洁" },
        { label: "模块", key: "module", value: "贴头系统" },
        { label: "周期", key: "interval", value: "8小时" },
        { label: "下次时间", key: "next", value: "今日 18:00" },
        { label: "状态", key: "status", value: "待执行" },
        { label: "处理建议", key: "action", value: "清洁后重新校准" }
    ]
    readonly property var logFields: [
        { label: "报警代码", key: "code", value: "VIS-004" },
        { label: "等级", key: "level", value: "警告" },
        { label: "模块", key: "module", value: "视觉" },
        { label: "描述", key: "desc", value: "Mark识别失败" },
        { label: "处理建议", key: "action", value: "清洁基准点并重新识别" },
        { label: "状态", key: "status", value: "未确认" }
    ]

    function mapX(x) { return image.x + x * image.paintedWidth / designWidth }
    function mapY(y) { return image.y + y * image.paintedHeight / designHeight }
    function mapW(w) { return w * image.paintedWidth / designWidth }
    function mapH(h) { return h * image.paintedHeight / designHeight }
    function pageFields() {
        return [productionFields, programFields, feederFields, headFields, visionFields, conveyorFields, motionFields, maintenanceFields, logFields][pageIndex]
    }
    function selectedRows() {
        if (!store) return []
        if (pageIndex === 0) return [{ currentProgram: store.currentProgram, boardCount: store.boardCount, cph: store.cph, cycleTime: store.cycleTime, yieldRate: store.yieldRate, machineState: store.machineState }]
        if (pageIndex === 1) return store.programRows
        if (pageIndex === 2) return store.feeders
        if (pageIndex === 3) return store.heads
        if (pageIndex === 4) return store.cameras
        if (pageIndex === 5) return store.rails
        if (pageIndex === 7) return store.maintenance
        if (pageIndex === 8) return store.alarms
        return []
    }
    function selectedIndex() {
        if (!store) return 0
        if (pageIndex === 1) return store.selectedProgramRow
        if (pageIndex === 2) return store.selectedFeeder
        if (pageIndex === 3) return store.selectedHead
        if (pageIndex === 4) return store.selectedCamera
        if (pageIndex === 5) return store.selectedRail
        if (pageIndex === 7) return store.selectedMaintenance
        if (pageIndex === 8) return store.selectedAlarm
        return 0
    }
    function defaultForFields(fields) {
        var row = {}
        for (var i = 0; i < fields.length; i++)
            row[fields[i].key] = fields[i].value || ""
        return row
    }

    Rectangle {
        anchors.fill: parent
        color: "#eef2f7"
    }

    Image {
        id: image
        anchors.fill: parent
        fillMode: Image.PreserveAspectFit
        source: root.source
        smooth: true
        mipmap: true
    }

    Rectangle {
        id: formOverlay
        anchors.fill: parent
        visible: false
        z: 20
        color: "#660f172a"

        Rectangle {
            width: Math.min(parent.width - 64, 660)
            height: Math.min(parent.height - 80, 640)
            anchors.centerIn: parent
            radius: 8
            color: "#ffffff"
            border.color: "#b9c4d2"

            ColumnLayout {
                anchors.fill: parent
                anchors.margins: 18
                spacing: 12

                RowLayout {
                    Layout.fillWidth: true
                    Label {
                        text: root.pageName + " - " + (root.editMode === "create" ? "新增" : "编辑")
                        font.pixelSize: 18
                        font.weight: Font.DemiBold
                        color: "#152033"
                        Layout.fillWidth: true
                    }
                    Button {
                        text: "关闭"
                        onClicked: formOverlay.visible = false
                    }
                }

                ScrollView {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    ColumnLayout {
                        width: Math.max(parent.width - 18, 420)
                        spacing: 10
                        Repeater {
                            id: fieldRepeater
                            model: root.activeFields
                            delegate: RowLayout {
                                required property int index
                                required property var modelData
                                property alias valueText: input.text
                                Layout.fillWidth: true
                                Label {
                                    text: modelData.label
                                    color: "#243044"
                                    font.pixelSize: 13
                                    Layout.preferredWidth: 108
                                    horizontalAlignment: Text.AlignRight
                                }
                                TextField {
                                    id: input
                                    Layout.fillWidth: true
                                    text: root.activeRecord[modelData.key] === undefined ? modelData.value : root.activeRecord[modelData.key]
                                    placeholderText: modelData.label
                                }
                            }
                        }
                    }
                }

                RowLayout {
                    Layout.fillWidth: true
                    Item { Layout.fillWidth: true }
                    Button {
                        text: "取消"
                        onClicked: formOverlay.visible = false
                    }
                    Button {
                        text: "保存"
                        highlighted: true
                        onClicked: {
                            root.saveRecord(root.editMode === "create")
                            machineBackend.executeCommand(root.pageName, root.editMode === "create" ? "新增保存" : "编辑保存")
                            formOverlay.visible = false
                        }
                    }
                }
            }
        }
    }

    Repeater {
        model: [
            { y: 110, page: 0 },
            { y: 190, page: 1 },
            { y: 270, page: 2 },
            { y: 350, page: 3 },
            { y: 430, page: 4 },
            { y: 510, page: 5 },
            { y: 590, page: 6 },
            { y: 670, page: 7 },
            { y: 750, page: 8 }
        ]
        delegate: MouseArea {
            x: root.mapX(0)
            y: root.mapY(modelData.y)
            width: root.mapW(82)
            height: root.mapH(58)
            onClicked: root.navigate(modelData.page)
            cursorShape: Qt.PointingHandCursor
        }
    }

    Repeater {
        model: [
            { label: "新增", x: 1340, y: 820, w: 86, h: 42, action: "add" },
            { label: "编辑", x: 1432, y: 820, w: 86, h: 42, action: "edit" },
            { label: "删除", x: 1524, y: 820, w: 86, h: 42, action: "delete" },
            { label: "保存", x: 1420, y: 875, w: 120, h: 42, action: "save" },
            { label: "启动", x: 1350, y: 780, w: 100, h: 46, action: "start" },
            { label: "暂停", x: 1460, y: 780, w: 100, h: 46, action: "pause" },
            { label: "停止", x: 1570, y: 780, w: 86, h: 46, action: "stop" }
        ]
        delegate: MouseArea {
            x: root.mapX(modelData.x)
            y: root.mapY(modelData.y)
            width: root.mapW(modelData.w)
            height: root.mapH(modelData.h)
            enabled: width > 20 && height > 12
            cursorShape: Qt.PointingHandCursor
            onClicked: root.handleAction(modelData.action)
        }
    }

    Dialog {
        id: crudDialog
        modal: true
        title: root.pageName + " - " + (root.editMode === "create" ? "新增" : "编辑")
        width: Math.min(root.width - 40, 620)
        x: (root.width - width) / 2
        y: Math.max(40, root.height * 0.12)
        standardButtons: Dialog.Ok | Dialog.Cancel
        contentItem: ScrollView {
            implicitHeight: Math.min(520, root.height - 180)
            ColumnLayout {
                width: crudDialog.width - 44
                spacing: 10
                Repeater {
                    id: dialogFieldRepeater
                    model: root.activeFields
                    delegate: RowLayout {
                        required property int index
                        required property var modelData
                        property alias valueText: input.text
                        Layout.fillWidth: true
                        Label {
                            text: modelData.label
                            color: "#243044"
                            font.pixelSize: 13
                            Layout.preferredWidth: 92
                            horizontalAlignment: Text.AlignRight
                        }
                        TextField {
                            id: input
                            Layout.fillWidth: true
                            text: root.activeRecord[modelData.key] === undefined ? modelData.value : root.activeRecord[modelData.key]
                            placeholderText: modelData.label
                        }
                    }
                }
            }
        }
        onAccepted: {
            root.saveRecord(root.editMode === "create")
        }
    }

    Dialog {
        id: confirmDialog
        modal: true
        property string actionName: ""
        title: root.pageName + " - " + actionName
        width: Math.min(root.width - 40, 420)
        x: (root.width - width) / 2
        y: Math.max(40, root.height * 0.16)
        standardButtons: Dialog.Ok | Dialog.Cancel
        Label {
            text: "确认执行 " + confirmDialog.actionName + "？"
            padding: 20
        }
        onAccepted: {
            if (root.store) {
                root.store.addAlarm({
                    time: "现在",
                    level: confirmDialog.actionName === "删除" || confirmDialog.actionName === "停止" ? "警告" : "提示",
                    code: "UI-ACT",
                    module: root.pageName,
                    desc: confirmDialog.actionName + "操作已执行",
                    action: "来自设计稿热区",
                    status: "已确认"
                })
            }
        }
    }

    function handleAction(action) {
        if (action === "add") {
            root.createRecord()
            return
        }
        if (action === "edit") {
            root.updateRecord()
            return
        }
        if (action === "save") {
            root.store.addAlarm({ time: "现在", level: "提示", code: "SAVE", module: root.pageName, desc: "页面数据已保存", action: "内存数据模型", status: "已确认" })
            machineBackend.executeCommand(root.pageName, "保存")
            return
        }
        if (action === "delete") {
            root.deleteRecord()
            return
        }
        confirmDialog.actionName = action === "start" ? "启动" : (action === "pause" ? "暂停" : "停止")
        machineBackend.executeCommand(root.pageName, confirmDialog.actionName)
        confirmDialog.open()
    }

    function createRecord() {
        editMode = "create"
        activeFields = pageFields()
        activeRecord = defaultForFields(activeFields)
        formOverlay.visible = true
    }

    function updateRecord() {
        editMode = "update"
        activeFields = pageFields()
        var rows = selectedRows()
        activeRecord = rows.length > 0 ? JSON.parse(JSON.stringify(rows[Math.min(selectedIndex(), rows.length - 1)])) : defaultForFields(activeFields)
        formOverlay.visible = true
    }

    function collectRecord() {
        var row = {}
        for (var i = 0; i < activeFields.length; i++) {
            var item = fieldRepeater.itemAt(i)
            row[activeFields[i].key] = item ? item.valueText : activeFields[i].value
        }
        return row
    }

    function saveRecord(isCreate) {
        if (!store) return
        var row = collectRecord()
        var idx = isCreate ? -1 : selectedIndex()
        if (pageIndex === 0) {
            store.currentProgram = row.currentProgram || store.currentProgram
            store.boardCount = parseInt(row.boardCount) || store.boardCount
            store.cph = parseInt(row.cph) || store.cph
            store.machineState = row.machineState || store.machineState
            store.changed()
        } else if (pageIndex === 1) {
            isCreate ? store.addProgramRow(row) : store.updateProgramRow(idx, row)
        } else if (pageIndex === 2) {
            delete row.rearFace
            isCreate ? store.addFeeder(row) : store.updateFeeder(idx, row)
        } else if (pageIndex === 3) {
            store.upsertHead(idx, row)
        } else if (pageIndex === 4) {
            delete row.markCamera
            delete row.largeCamera
            store.upsertCamera(idx, row)
        } else if (pageIndex === 5) {
            var tray = { tray: row.tray || "Tray-01", material: row.board || "PCB", pockets: 24, used: 0, status: row.status || "正常" }
            delete row.mountArea
            delete row.outArea
            delete row.tray
            store.upsertRail(idx, row)
            if (isCreate) store.upsertTray(-1, tray)
        } else if (pageIndex === 6) {
            store.addAlarm({ time: "现在", level: "提示", code: "MOTION", module: "运动", desc: "轴参数已保存 X轴/Y轴/Z轴", action: row.servo || "使能", status: "已确认" })
        } else if (pageIndex === 7) {
            store.upsertMaintenance(idx, row)
        } else {
            row.time = row.time || "现在"
            isCreate ? store.addAlarm(row) : (store.alarms[idx] = row, store.alarms = store.cloneArray(store.alarms), store.changed())
        }
    }

    function deleteRecord() {
        if (!store) return
        if (pageIndex === 1) store.deleteProgramRow(store.selectedProgramRow)
        else if (pageIndex === 2) store.deleteFeeder(store.selectedFeeder)
        else if (pageIndex === 3) store.deleteHead(store.selectedHead)
        else if (pageIndex === 4) store.deleteCamera(store.selectedCamera)
        else if (pageIndex === 5) store.deleteRail(store.selectedRail)
        else if (pageIndex === 7) store.deleteMaintenance(store.selectedMaintenance)
        else if (pageIndex === 8) store.clearAlarm(store.selectedAlarm)
        machineBackend.executeCommand(root.pageName, "删除")
        store.addAlarm({ time: "现在", level: "警告", code: "DELETE", module: root.pageName, desc: "删除/清除操作已执行", action: "来自设计稿热区", status: "已确认" })
    }
}

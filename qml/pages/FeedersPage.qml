import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../components"

Item {
    property var store
    property bool compact: false
    property string filter: "全部"

    function filteredFeeders() {
        if (!store) return []
        if (filter === "全部") return store.feeders
        if (filter === "已装料") return store.feeders.filter(function(f) { return f.status === "正常" })
        if (filter === "缺料") return store.feeders.filter(function(f) { return f.status === "缺料" })
        if (filter === "报警") return store.feeders.filter(function(f) { return f.status !== "正常" })
        return store.feeders.filter(function(f) { return f.status === "待校验" })
    }

    Dialog {
        id: feederDialog
        modal: true
        title: editIndex < 0 ? "新增飞达" : "编辑飞达"
        property int editIndex: -1
        width: 420
        x: (parent.width - width) / 2
        y: 70
        standardButtons: Dialog.Ok | Dialog.Cancel
        contentItem: GridLayout {
            columns: 2
            Label { text: "槽位" }
            TextField { id: slotField; placeholderText: "01" }
            Label { text: "面" }
            ComboBox { id: faceField; model: ["前", "后"] }
            Label { text: "物料" }
            TextField { id: materialField; placeholderText: "R0603-10K" }
            Label { text: "封装" }
            TextField { id: packageField; placeholderText: "0603" }
            Label { text: "间距" }
            TextField { id: pitchField; placeholderText: "4" }
            Label { text: "数量" }
            TextField { id: qtyField; placeholderText: "5000" }
            Label { text: "贴头" }
            TextField { id: headField; placeholderText: "H1" }
            Label { text: "吸嘴" }
            TextField { id: nozzleField; placeholderText: "CN040" }
        }
        onAccepted: {
            var row = { slot: slotField.text, face: faceField.currentText, material: materialField.text, packageName: packageField.text, pitch: pitchField.text, qty: qtyField.text, head: headField.text, nozzle: nozzleField.text, status: "正常" }
            if (editIndex < 0) store.addFeeder(row); else store.updateFeeder(editIndex, row)
        }
        function openFor(index) {
            editIndex = index
            var row = index >= 0 && store ? store.feeders[index] : { slot: "", face: "前", material: "", packageName: "", pitch: "", qty: "", head: "", nozzle: "" }
            slotField.text = row.slot; faceField.currentIndex = row.face === "后" ? 1 : 0; materialField.text = row.material; packageField.text = row.packageName; pitchField.text = row.pitch; qtyField.text = row.qty; headField.text = row.head; nozzleField.text = row.nozzle
            open()
        }
    }

    RowLayout {
        anchors.fill: parent
        spacing: 10

        Section {
            title: "前60飞达 / 后60飞达 栈位图"
            Layout.fillWidth: true
            Layout.fillHeight: true
            ColumnLayout {
                anchors.fill: parent
                RowLayout {
                    Repeater {
                        model: ["全部", "已装料", "缺料", "报警", "待校验"]
                        delegate: Button { text: modelData; checkable: true; checked: filter === modelData; onClicked: filter = modelData }
                    }
                    Item { Layout.fillWidth: true }
                    Button { text: "扫码绑定"; onClicked: feederDialog.openFor(-1) }
                    Button { text: "飞达校准"; onClicked: store.addAlarm({ time: "现在", level: "提示", code: "FDR-CAL", module: "飞达", desc: "飞达校准任务已创建", action: "执行校准", status: "未确认" }) }
                }
                GridLayout {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 170
                    columns: 60
                    rowSpacing: 2
                    columnSpacing: 2
                    Repeater {
                        model: 120
                        delegate: Rectangle {
                            Layout.fillWidth: true
                            Layout.preferredHeight: 18
                            radius: 2
                            color: (store && store.feeders[index] && store.feeders[index].status === "缺料") ? "#ef4444" : ((store && store.feeders[index] && store.feeders[index].status === "待校验") ? "#f59e0b" : "#38bdf8")
                            Text { anchors.centerIn: parent; text: (index % 60) + 1; font.pixelSize: 8; color: "white" }
                            MouseArea { anchors.fill: parent; onClicked: store.selectedFeeder = index; onDoubleClicked: feederDialog.openFor(index) }
                        }
                    }
                }
                DataTable {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    rows: filteredFeeders()
                    selectedIndex: store ? store.selectedFeeder : -1
                    onRowSelected: store.selectedFeeder = index
                    onEditRequested: feederDialog.openFor(index)
                    columns: [
                        { title: "槽位", key: "slot", width: 52 },
                        { title: "面", key: "face", width: 36 },
                        { title: "物料", key: "material", width: 110 },
                        { title: "封装", key: "packageName", width: 64 },
                        { title: "数量", key: "qty", width: 72 },
                        { title: "贴头", key: "head", width: 48 },
                        { title: "吸嘴", key: "nozzle", width: 70 },
                        { title: "状态", key: "status", fill: true }
                    ]
                }
            }
        }

        Section {
            title: "托盘与当前槽位"
            Layout.preferredWidth: compact ? 250 : 330
            Layout.fillHeight: true
            ColumnLayout {
                anchors.fill: parent
                DataTable {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 150
                    rows: store ? store.trays : []
                    columns: [
                        { title: "托盘", key: "tray", width: 70 },
                        { title: "物料", key: "material", fill: true },
                        { title: "状态", key: "status", width: 70 }
                    ]
                }
                Label { text: store && store.feeders[store.selectedFeeder] ? "当前: " + store.feeders[store.selectedFeeder].face + store.feeders[store.selectedFeeder].slot + " / " + store.feeders[store.selectedFeeder].material : "未选择"; wrapMode: Text.WordWrap }
                Button { text: "新增"; Layout.fillWidth: true; onClicked: feederDialog.openFor(-1) }
                Button { text: "编辑"; Layout.fillWidth: true; onClicked: feederDialog.openFor(store.selectedFeeder) }
                Button { text: "删除"; Layout.fillWidth: true; onClicked: store.deleteFeeder(store.selectedFeeder) }
                Button { text: "保存"; Layout.fillWidth: true; onClicked: store.addAlarm({ time: "现在", level: "提示", code: "FDR-SAVE", module: "飞达", desc: "飞达清单已保存", action: "无", status: "已确认" }) }
                Item { Layout.fillHeight: true }
            }
        }
    }
}

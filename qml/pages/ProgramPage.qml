import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../components"

Item {
    property var store
    property bool compact: false

    Dialog {
        id: editor
        modal: true
        title: editIndex < 0 ? "新增贴装记录" : "编辑贴装记录"
        property int editIndex: -1
        standardButtons: Dialog.Ok | Dialog.Cancel
        width: 420
        x: (parent.width - width) / 2
        y: 80
        contentItem: GridLayout {
            columns: 2
            TextField { id: refField; placeholderText: "位号" }
            TextField { id: materialField; placeholderText: "物料" }
            TextField { id: sourceField; placeholderText: "前-01 / 托盘-01" }
            TextField { id: headField; placeholderText: "H1-H8" }
            TextField { id: nozzleField; placeholderText: "CN040" }
            TextField { id: xField; placeholderText: "X坐标" }
            TextField { id: yField; placeholderText: "Y坐标" }
            TextField { id: angleField; placeholderText: "角度" }
        }
        onAccepted: {
            var row = { ref: refField.text, material: materialField.text, source: sourceField.text, head: headField.text, nozzle: nozzleField.text, x: xField.text, y: yField.text, angle: angleField.text, strategy: "飞行相机", status: "待校验" }
            if (editIndex < 0) store.addProgramRow(row); else store.updateProgramRow(editIndex, row)
        }
        function openFor(index) {
            editIndex = index
            var row = index >= 0 && store ? store.programRows[index] : { ref: "", material: "", source: "", head: "", nozzle: "", x: "", y: "", angle: "" }
            refField.text = row.ref; materialField.text = row.material; sourceField.text = row.source; headField.text = row.head; nozzleField.text = row.nozzle; xField.text = row.x; yField.text = row.y; angleField.text = row.angle
            open()
        }
    }

    RowLayout {
        anchors.fill: parent
        spacing: 10

        Section {
            title: "PCB程序结构"
            Layout.preferredWidth: compact ? 170 : 220
            Layout.fillHeight: true
            ListView {
                anchors.fill: parent
                model: ["PCB工程", "元件库", "贴装步骤", "拼板设置", "坐标文件", "视觉策略", "路径优化"]
                delegate: ItemDelegate { width: parent.width; text: modelData; highlighted: index === 2 }
            }
        }

        Section {
            title: "PCB坐标编辑"
            Layout.fillWidth: true
            Layout.fillHeight: true
            Rectangle {
                anchors.fill: parent
                color: "#e0f2fe"
                border.color: "#38bdf8"
                Repeater {
                    model: store ? store.programRows : []
                    delegate: Rectangle {
                        x: 30 + (index * 52) % Math.max(80, parent.width - 80)
                        y: 36 + (index * 37) % Math.max(80, parent.height - 80)
                        width: modelData.ref.indexOf("U") === 0 ? 42 : 24
                        height: modelData.ref.indexOf("U") === 0 ? 42 : 18
                        radius: 2
                        color: index === (store ? store.selectedProgramRow : -1) ? "#f97316" : "#2563eb"
                        Text { anchors.centerIn: parent; text: modelData.ref; color: "white"; font.pixelSize: 10 }
                        MouseArea { anchors.fill: parent; onClicked: store.selectedProgramRow = index; onDoubleClicked: editor.openFor(index) }
                    }
                }
                Text { anchors.left: parent.left; anchors.top: parent.top; anchors.margins: 8; text: "原点 / Mark点 / 元件坐标"; color: "#075985" }
            }
        }

        Section {
            title: "贴装表"
            Layout.preferredWidth: compact ? 286 : 460
            Layout.fillHeight: true
            ColumnLayout {
                anchors.fill: parent
                DataTable {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    rows: store ? store.programRows : []
                    selectedIndex: store ? store.selectedProgramRow : -1
                    onRowSelected: store.selectedProgramRow = index
                    onEditRequested: editor.openFor(index)
                    columns: [
                        { title: "位号", key: "ref", width: 58 },
                        { title: "物料", key: "material", width: 100 },
                        { title: "飞达", key: "source", width: 70 },
                        { title: "贴头", key: "head", width: 46 },
                        { title: "状态", key: "status", fill: true }
                    ]
                }
                GridLayout {
                    Layout.fillWidth: true
                    columns: compact ? 2 : 4
                    Button { text: "新增"; Layout.fillWidth: true; onClicked: editor.openFor(-1) }
                    Button { text: "编辑"; Layout.fillWidth: true; onClicked: editor.openFor(store.selectedProgramRow) }
                    Button { text: "删除"; Layout.fillWidth: true; onClicked: store.deleteProgramRow(store.selectedProgramRow) }
                    Button { text: "保存"; Layout.fillWidth: true; onClicked: store.addAlarm({ time: "现在", level: "提示", code: "PGM", module: "程序", desc: "程序已保存", action: "无", status: "已确认" }) }
                }
            }
        }
    }
}

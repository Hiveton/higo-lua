import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../components"

Item {
    property var store
    property bool compact: false

    Dialog {
        id: alarmDialog
        modal: true
        title: "新增报警"
        width: 420
        x: (parent.width - width) / 2
        y: 80
        standardButtons: Dialog.Ok | Dialog.Cancel
        contentItem: GridLayout {
            columns: 2
            Label { text: "等级" }
            ComboBox { id: levelField; model: ["提示", "警告", "严重"] }
            Label { text: "代码" }
            TextField { id: codeField; text: "USR-001" }
            Label { text: "模块" }
            TextField { id: moduleField; text: "生产" }
            Label { text: "描述" }
            TextField { id: descField; placeholderText: "报警描述" }
            Label { text: "处理建议" }
            TextField { id: actionField; placeholderText: "处理建议" }
        }
        onAccepted: store.addAlarm({ time: "现在", level: levelField.currentText, code: codeField.text, module: moduleField.text, desc: descField.text, action: actionField.text, status: "未确认" })
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 10

        GridLayout {
            Layout.fillWidth: true
            Layout.preferredHeight: 88
            columns: compact ? 3 : 6
            Repeater {
                model: [
                    { k: "当前报警", v: store ? store.alarms.filter(function(a) { return a.status !== "已确认" }).length : 0 },
                    { k: "今日报警", v: store ? store.alarms.length : 0 },
                    { k: "已处理", v: store ? store.alarms.filter(function(a) { return a.status === "已确认" }).length : 0 },
                    { k: "停机报警", v: 1 },
                    { k: "良率", v: store ? store.yieldRate + "%" : "0%" },
                    { k: "产量", v: store ? store.boardCount : 0 }
                ]
                delegate: Section {
                    title: modelData.k
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    Label { anchors.centerIn: parent; text: modelData.v; color: "#1d4ed8"; font.pixelSize: 22; font.weight: Font.DemiBold }
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 10
            Section {
                title: "报警日志 / 操作记录 / 生产追溯"
                Layout.fillWidth: true
                Layout.fillHeight: true
                ColumnLayout {
                    anchors.fill: parent
                    TabBar {
                        Layout.fillWidth: true
                        Repeater {
                            model: ["当前报警", "历史报警", "操作记录", "生产记录", "追溯"]
                            delegate: TabButton { text: modelData }
                        }
                    }
                    DataTable {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        rows: store ? store.alarms : []
                        selectedIndex: store ? store.selectedAlarm : -1
                        onRowSelected: store.selectedAlarm = index
                        columns: [
                            { title: "时间", key: "time", width: 76 },
                            { title: "等级", key: "level", width: 58 },
                            { title: "代码", key: "code", width: 78 },
                            { title: "模块", key: "module", width: 70 },
                            { title: "描述", key: "desc", fill: true },
                            { title: "处理建议", key: "action", width: 150 },
                            { title: "状态", key: "status", width: 80 }
                        ]
                    }
                }
            }
            Section {
                title: "报警详情"
                Layout.preferredWidth: compact ? 300 : 380
                Layout.fillHeight: true
                ColumnLayout {
                    anchors.fill: parent
                    Label { Layout.fillWidth: true; wrapMode: Text.WordWrap; text: store && store.alarms[store.selectedAlarm] ? store.alarms[store.selectedAlarm].code + " / " + store.alarms[store.selectedAlarm].desc : "未选择"; font.pixelSize: 15; font.weight: Font.DemiBold }
                    Label { Layout.fillWidth: true; wrapMode: Text.WordWrap; text: store && store.alarms[store.selectedAlarm] ? "处理建议: " + store.alarms[store.selectedAlarm].action : "" }
                    Rectangle { Layout.fillWidth: true; Layout.preferredHeight: 140; color: "#0f172a"; radius: 4; Label { anchors.centerIn: parent; text: "相关截图 / 波形 / 视觉结果"; color: "#cbd5e1" } }
                    Button { text: "新增"; Layout.fillWidth: true; onClicked: alarmDialog.open() }
                    Button { text: "确认报警"; Layout.fillWidth: true; onClicked: store.clearAlarm(store.selectedAlarm) }
                    Button { text: "清除已处理"; Layout.fillWidth: true; onClicked: store.clearAlarm(store.selectedAlarm) }
                    Button { text: "导出日志"; Layout.fillWidth: true }
                    Button { text: "导出追溯"; Layout.fillWidth: true }
                    Button { text: "打印报表"; Layout.fillWidth: true }
                    Item { Layout.fillHeight: true }
                }
            }
        }
    }
}

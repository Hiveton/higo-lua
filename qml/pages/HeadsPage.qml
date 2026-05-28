import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../components"

Item {
    property var store
    property bool compact: false

    RowLayout {
        anchors.fill: parent
        spacing: 10
        Section {
            title: "8贴头与吸嘴状态"
            Layout.fillWidth: true
            Layout.fillHeight: true
            ColumnLayout {
                anchors.fill: parent
                GridLayout {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 150
                    columns: compact ? 4 : 8
                    Repeater {
                        model: store ? store.heads : []
                        delegate: Rectangle {
                            Layout.fillWidth: true
                            Layout.fillHeight: true
                            radius: 6
                            color: modelData.status === "正常" ? "#dcfce7" : "#fef3c7"
                            border.color: modelData.status === "正常" ? "#22c55e" : "#f59e0b"
                            Column {
                                anchors.centerIn: parent
                                spacing: 4
                                Label { text: modelData.id; font.pixelSize: 18; font.weight: Font.DemiBold; color: "#064e3b" }
                                Label { text: modelData.nozzle; font.pixelSize: 12 }
                                Label { text: modelData.vacuum + " kPa"; font.pixelSize: 12 }
                                Label { text: modelData.status; font.pixelSize: 12 }
                            }
                        }
                    }
                }
                DataTable {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    rows: store ? store.heads : []
                    columns: [
                        { title: "贴头", key: "id", width: 60 },
                        { title: "吸嘴", key: "nozzle", width: 90 },
                        { title: "状态", key: "status", width: 80 },
                        { title: "真空(kPa)", key: "vacuum", width: 90 },
                        { title: "Z(mm)", key: "z", width: 80 },
                        { title: "角度", key: "theta", width: 70 },
                        { title: "取料成功率", key: "success", fill: true }
                    ]
                }
            }
        }
        Section {
            title: "吸嘴库 / 维护操作"
            Layout.preferredWidth: compact ? 260 : 340
            Layout.fillHeight: true
            ColumnLayout {
                anchors.fill: parent
                GridLayout {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 140
                    columns: 4
                    Repeater {
                        model: 12
                        delegate: Rectangle {
                            Layout.fillWidth: true
                            Layout.fillHeight: true
                            radius: 4
                            color: index % 4 === 0 ? "#fef3c7" : "#dbeafe"
                            Text { anchors.centerIn: parent; text: "N" + (501 + index); font.pixelSize: 11; color: "#1e3a8a" }
                        }
                    }
                }
                Repeater {
                    model: ["吸嘴更换", "真空测试", "高度校准", "旋转校准", "贴头回零", "禁用贴头"]
                    delegate: Button { Layout.fillWidth: true; text: modelData; onClicked: store.addAlarm({ time: "现在", level: "提示", code: "HEAD", module: "贴头", desc: modelData + "已下发", action: "等待完成", status: "未确认" }) }
                }
                Item { Layout.fillHeight: true }
            }
        }
    }
}

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
            title: "三段式轨道与托盘"
            Layout.fillWidth: true
            Layout.fillHeight: true
            ColumnLayout {
                anchors.fill: parent
                Rectangle {
                    Layout.fillWidth: true
                    Layout.preferredHeight: compact ? 220 : 300
                    radius: 6
                    color: "#f8fafc"
                    border.color: "#cbd5e1"
                    Row {
                        anchors.centerIn: parent
                        spacing: 12
                        Repeater {
                            model: store ? store.rails : []
                            delegate: Rectangle {
                                width: Math.max(150, parent.parent.width * 0.24)
                                height: 110
                                radius: 6
                                color: index === 1 ? "#dbeafe" : "#f1f5f9"
                                border.color: "#64748b"
                                Column {
                                    anchors.centerIn: parent
                                    spacing: 4
                                    Label { text: modelData.area; font.pixelSize: 18; font.weight: Font.DemiBold; color: "#1e3a8a" }
                                    Label { text: "PCB " + modelData.board; font.pixelSize: 12 }
                                    Label { text: "宽度 " + modelData.width + " mm"; font.pixelSize: 12 }
                                    Label { text: modelData.status; font.pixelSize: 12; color: "#166534" }
                                }
                            }
                        }
                    }
                }
                DataTable {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    rows: store ? store.rails : []
                    columns: [
                        { title: "区域", key: "area", width: 90 },
                        { title: "PCB编号", key: "board", width: 100 },
                        { title: "宽度", key: "width", width: 70 },
                        { title: "速度", key: "speed", width: 70 },
                        { title: "传感器", key: "sensor", width: 80 },
                        { title: "状态", key: "status", fill: true }
                    ]
                }
            }
        }
        Section {
            title: "托盘与轨道动作"
            Layout.preferredWidth: compact ? 280 : 360
            Layout.fillHeight: true
            ColumnLayout {
                anchors.fill: parent
                DataTable {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 160
                    rows: store ? store.trays : []
                    columns: [
                        { title: "托盘", key: "tray", width: 70 },
                        { title: "物料", key: "material", fill: true },
                        { title: "状态", key: "status", width: 70 }
                    ]
                }
                GridLayout {
                    Layout.fillWidth: true
                    columns: 2
                    Repeater {
                        model: ["调宽", "进板", "出板", "夹紧", "松开", "轨道回零", "托盘校准", "托盘换料"]
                        delegate: Button { Layout.fillWidth: true; text: modelData; onClicked: store.addAlarm({ time: "现在", level: "提示", code: "RAIL", module: "轨道", desc: modelData + "已执行", action: "无", status: "已确认" }) }
                    }
                }
                Item { Layout.fillHeight: true }
            }
        }
    }
}

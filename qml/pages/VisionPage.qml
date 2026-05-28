import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../components"

Item {
    property var store
    property bool compact: false

    GridLayout {
        anchors.fill: parent
        columns: compact ? 1 : 3
        rowSpacing: 10
        columnSpacing: 10

        Repeater {
            model: store ? store.cameras : []
            delegate: Section {
                title: modelData.name
                Layout.fillWidth: true
                Layout.fillHeight: true
                Rectangle {
                    anchors.fill: parent
                    color: "#0f172a"
                    radius: 4
                    Repeater {
                        model: 18
                        delegate: Rectangle {
                            x: 20 + Math.random() * (parent.width - 60)
                            y: 20 + Math.random() * (parent.height - 60)
                            width: 24 + Math.random() * 38
                            height: 16 + Math.random() * 30
                            color: "transparent"
                            border.color: index % 3 === 0 ? "#22c55e" : "#38bdf8"
                        }
                    }
                    Rectangle { anchors.centerIn: parent; width: parent.width * 0.78; height: 1; color: "#f97316" }
                    Rectangle { anchors.centerIn: parent; width: 1; height: parent.height * 0.78; color: "#f97316" }
                    Label { anchors.left: parent.left; anchors.top: parent.top; anchors.margins: 8; text: modelData.mode + " / " + modelData.status; color: "#bbf7d0" }
                }
            }
        }

        Section {
            title: "视觉策略与相机参数"
            Layout.columnSpan: compact ? 1 : 2
            Layout.fillWidth: true
            Layout.preferredHeight: 220
            DataTable {
                anchors.fill: parent
                rows: store ? store.cameras : []
                columns: [
                    { title: "名称", key: "name", width: 100 },
                    { title: "算法", key: "mode", width: 110 },
                    { title: "曝光", key: "exposure", width: 70 },
                    { title: "增益", key: "gain", width: 60 },
                    { title: "光源", key: "light", width: 120 },
                    { title: "偏移", key: "offset", width: 110 },
                    { title: "状态", key: "status", fill: true }
                ]
            }
        }

        Section {
            title: "相机操作"
            Layout.fillWidth: true
            Layout.preferredHeight: 220
            GridLayout {
                anchors.fill: parent
                columns: 2
                Repeater {
                    model: ["采集图像", "自动曝光", "标定相机", "保存策略", "测试识别", "复位光源"]
                    delegate: Button {
                        Layout.fillWidth: true
                        Layout.preferredHeight: 44
                        text: modelData
                        onClicked: store.addAlarm({ time: "现在", level: "提示", code: "VIS", module: "视觉", desc: modelData + "完成", action: "检查结果", status: "已确认" })
                    }
                }
            }
        }
    }
}

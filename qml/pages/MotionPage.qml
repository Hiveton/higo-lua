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
            title: "坐标与点动"
            Layout.fillWidth: true
            Layout.fillHeight: true
            ColumnLayout {
                anchors.fill: parent
                MachineMap { Layout.fillWidth: true; Layout.fillHeight: true }
                RowLayout {
                    Layout.fillWidth: true
                    Repeater {
                        model: ["0.01", "0.1", "1", "10"]
                        delegate: Button { text: modelData + " mm"; checkable: true; checked: index === 2 }
                    }
                    Slider { Layout.fillWidth: true; from: 1; to: 100; value: 35 }
                }
            }
        }
        Section {
            title: "轴状态 / 校准"
            Layout.preferredWidth: compact ? 330 : 440
            Layout.fillHeight: true
            ColumnLayout {
                anchors.fill: parent
                GridLayout {
                    Layout.fillWidth: true
                    columns: 3
                    Repeater {
                        model: ["X-", "Y+", "X+", "Z+", "回零", "Z-", "R-", "Y-", "R+"]
                        delegate: Button { Layout.fillWidth: true; Layout.preferredHeight: 42; text: modelData }
                    }
                }
                DataTable {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    rows: [
                        { axis: "X轴", pos: "128.420", target: "130.000", servo: "使能", limit: "正常" },
                        { axis: "Y轴", pos: "64.120", target: "64.120", servo: "使能", limit: "正常" },
                        { axis: "Z1-Z8", pos: "12.20", target: "12.20", servo: "使能", limit: "正常" },
                        { axis: "R1-R8", pos: "0-315", target: "路径角度", servo: "使能", limit: "正常" },
                        { axis: "轨道宽度", pos: "142.0", target: "142.0", servo: "使能", limit: "正常" },
                        { axis: "托盘Z", pos: "4.2", target: "4.2", servo: "使能", limit: "正常" }
                    ]
                    columns: [
                        { title: "轴", key: "axis", width: 80 },
                        { title: "当前位置", key: "pos", width: 90 },
                        { title: "目标", key: "target", width: 90 },
                        { title: "伺服", key: "servo", width: 70 },
                        { title: "限位", key: "limit", fill: true }
                    ]
                }
                RowLayout {
                    Repeater {
                        model: ["全轴回零", "伺服使能", "保存坐标", "安全复位"]
                        delegate: Button { text: modelData; Layout.fillWidth: true; onClicked: store.addAlarm({ time: "现在", level: "提示", code: "MOT", module: "运动", desc: modelData + "完成", action: "无", status: "已确认" }) }
                    }
                }
            }
        }
    }
}

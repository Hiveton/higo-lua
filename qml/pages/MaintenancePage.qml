import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../components"

Item {
    property var store
    property bool compact: false

    ColumnLayout {
        anchors.fill: parent
        spacing: 10
        GridLayout {
            Layout.fillWidth: true
            Layout.preferredHeight: compact ? 110 : 130
            columns: compact ? 4 : 7
            Repeater {
                model: ["贴头系统", "飞达系统", "视觉系统", "轨道系统", "气路系统", "伺服系统", "工控机"]
                delegate: Section {
                    title: modelData
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    Label {
                        anchors.centerIn: parent
                        text: index === 0 ? "92%\n待保养" : "99%\nOK"
                        horizontalAlignment: Text.AlignHCenter
                        color: index === 0 ? "#b45309" : "#166534"
                        font.pixelSize: 18
                        font.weight: Font.DemiBold
                    }
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 10
            Section {
                title: "I/O诊断"
                Layout.fillWidth: true
                Layout.fillHeight: true
                GridLayout {
                    anchors.fill: parent
                    columns: compact ? 3 : 5
                    Repeater {
                        model: ["急停", "安全门", "负压", "气压", "进板传感器", "出板传感器", "光源", "真空阀", "轨道电机", "飞达脉冲", "托盘升降", "相机触发", "伺服报警", "原点信号", "SMEMA"]
                        delegate: Rectangle {
                            Layout.fillWidth: true
                            Layout.preferredHeight: 42
                            radius: 4
                            color: index === 12 ? "#fee2e2" : "#ecfdf5"
                            border.color: index === 12 ? "#ef4444" : "#22c55e"
                            Label { anchors.centerIn: parent; text: modelData; color: "#334155"; font.pixelSize: 12 }
                        }
                    }
                }
            }
            Section {
                title: "维护清单与动作"
                Layout.preferredWidth: compact ? 320 : 440
                Layout.fillHeight: true
                ColumnLayout {
                    anchors.fill: parent
                    DataTable {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        rows: store ? store.maintenance : []
                        columns: [
                            { title: "项目", key: "item", width: 90 },
                            { title: "模块", key: "module", width: 90 },
                            { title: "周期", key: "interval", width: 70 },
                            { title: "下次", key: "next", width: 80 },
                            { title: "状态", key: "status", fill: true }
                        ]
                    }
                    GridLayout {
                        Layout.fillWidth: true
                        columns: 2
                        Repeater {
                            model: ["自检", "I/O测试", "备份数据", "恢复参数", "导出诊断", "工程师模式"]
                            delegate: Button { Layout.fillWidth: true; text: modelData; onClicked: store.addAlarm({ time: "现在", level: "提示", code: "MNT", module: "维护", desc: modelData + "已执行", action: "无", status: "已确认" }) }
                        }
                    }
                }
            }
        }
    }
}

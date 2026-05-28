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

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 10

            Section {
                title: "生产监控"
                Layout.fillWidth: true
                Layout.fillHeight: true
                MachineMap { anchors.fill: parent }
            }

            Section {
                title: "当前任务"
                Layout.preferredWidth: compact ? 270 : 360
                Layout.fillHeight: true
                ColumnLayout {
                    anchors.fill: parent
                    spacing: 8
                    Repeater {
                        model: [
                            "当前PCB: PCB-1287",
                            "下一物料: R0603-10K",
                            "取料槽位: 前-01",
                            "贴头: H1 / CN040",
                            "视觉: 飞行相机 OK",
                            "Mark: Mark相机 OK",
                            "大物件: 大物件相机 待机"
                        ]
                        delegate: Label { Layout.fillWidth: true; text: modelData; font.pixelSize: 13; color: "#334155"; elide: Text.ElideRight }
                    }
                    DataTable {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        rows: store ? store.heads : []
                        columns: [
                            { title: "贴头", key: "id", width: 46 },
                            { title: "吸嘴", key: "nozzle", width: 72 },
                            { title: "状态", key: "status", fill: true }
                        ]
                    }
                }
            }
        }

        Section {
            title: "流程与操作"
            Layout.fillWidth: true
            Layout.preferredHeight: compact ? 118 : 132
            RowLayout {
                anchors.fill: parent
                spacing: 10
                DataTable {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    rows: store ? store.alarms : []
                    columns: [
                        { title: "时间", key: "time", width: 70 },
                        { title: "等级", key: "level", width: 56 },
                        { title: "模块", key: "module", width: 60 },
                        { title: "描述", key: "desc", fill: true }
                    ]
                }
                GridLayout {
                    Layout.preferredWidth: compact ? 220 : 360
                    Layout.fillHeight: true
                    columns: 3
                    Repeater {
                        model: ["启动", "暂停", "停止", "复位", "原点", "DryRun"]
                        delegate: Button {
                            text: modelData
                            Layout.fillWidth: true
                            Layout.preferredHeight: compact ? 34 : 42
                            onClicked: if (store) store.addAlarm({ time: "现在", level: "提示", code: "OPR", module: "生产", desc: modelData + "操作已执行", action: "无", status: "已确认" })
                        }
                    }
                }
            }
        }
    }
}

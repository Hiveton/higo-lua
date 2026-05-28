import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Rectangle {
    id: root
    property var store
    readonly property bool compact: width < 980
    height: 56
    color: "#152033"

    RowLayout {
        anchors.fill: parent
        anchors.leftMargin: 16
        anchors.rightMargin: 16
        spacing: 14

        Label {
            text: "贴片机上位机控制系统"
            color: "white"
            font.pixelSize: 18
            font.weight: Font.DemiBold
            Layout.preferredWidth: root.compact ? 170 : 220
        }

        Repeater {
            model: [
                "状态 " + (root.store ? root.store.machineState : "-"),
                "程序 " + (root.store ? root.store.currentProgram : "-"),
                "产量 " + (root.store ? root.store.boardCount : 0),
                "CPH " + (root.store ? root.store.cph : 0),
                "周期 " + (root.store ? root.store.cycleTime : 0) + "s",
                "良率 " + (root.store ? root.store.yieldRate : 0) + "%"
            ]
            delegate: Rectangle {
                visible: !root.compact || index < 4
                Layout.preferredWidth: visible ? Math.max(root.compact ? 76 : 92, label.implicitWidth + 18) : 0
                Layout.preferredHeight: 30
                radius: 4
                color: index === 0 ? "#1f7a49" : "#24344f"
                Label {
                    id: label
                    anchors.centerIn: parent
                    text: modelData
                    color: "#edf4ff"
                    font.pixelSize: 12
                }
            }
        }

        Item { Layout.fillWidth: true }

        Rectangle {
            visible: !root.compact
            Layout.preferredWidth: visible ? 86 : 0
            Layout.preferredHeight: 30
            radius: 4
            color: "#783c18"
            Label {
                anchors.centerIn: parent
                text: "报警 2"
                color: "#ffd9b8"
                font.pixelSize: 12
                font.weight: Font.DemiBold
            }
        }
    }
}

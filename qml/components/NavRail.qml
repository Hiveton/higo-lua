import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Rectangle {
    id: root
    property int currentIndex: 0
    signal navigate(int index)
    width: 92
    color: "#101827"

    readonly property var items: ["生产", "程序", "飞达", "贴头", "视觉", "轨道", "运动", "维护", "日志"]

    ColumnLayout {
        anchors.fill: parent
        anchors.topMargin: 14
        anchors.bottomMargin: 14
        spacing: 6

        Repeater {
            model: root.items
            delegate: Button {
                Layout.fillWidth: true
                Layout.leftMargin: 8
                Layout.rightMargin: 8
                Layout.preferredHeight: 44
                text: modelData
                checkable: true
                checked: root.currentIndex === index
                onClicked: root.navigate(index)
                font.pixelSize: 14

                background: Rectangle {
                    radius: 6
                    color: parent.checked ? "#2563eb" : (parent.hovered ? "#1d293d" : "transparent")
                }
                contentItem: Label {
                    text: parent.text
                    color: parent.checked ? "white" : "#aeb9cc"
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                    font: parent.font
                }
            }
        }

        Item { Layout.fillHeight: true }
    }
}

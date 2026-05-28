import QtQuick
import QtQuick.Controls

Frame {
    id: root
    property string title: ""
    default property alias content: body.data
    property int pad: 10

    padding: pad
    background: Rectangle {
        color: "#ffffff"
        radius: 6
        border.color: "#d7dee8"
    }

    Column {
        anchors.fill: parent
        spacing: 8

        Label {
            text: root.title
            color: "#253040"
            font.pixelSize: 14
            font.weight: Font.DemiBold
            elide: Text.ElideRight
        }

        Item {
            id: body
            width: parent.width
            height: parent.height - y
        }
    }
}

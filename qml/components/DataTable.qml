import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    property var columns: []
    property var rows: []
    property int selectedIndex: -1
    signal rowSelected(int index)
    signal editRequested(int index)
    clip: true

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 34
            color: "#edf2f7"
            RowLayout {
                anchors.fill: parent
                spacing: 0
                Repeater {
                    model: root.columns
                    delegate: Label {
                        Layout.preferredWidth: modelData.width || 90
                        Layout.fillWidth: modelData.fill || false
                        text: modelData.title
                        color: "#334155"
                        font.pixelSize: 12
                        font.weight: Font.DemiBold
                        leftPadding: 8
                        verticalAlignment: Text.AlignVCenter
                        elide: Text.ElideRight
                    }
                }
            }
        }

        ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true

            Column {
                width: Math.max(parent.width, implicitWidth)
                Repeater {
                    model: root.rows
                    delegate: Rectangle {
                        id: row
                        property int rowIndex: index
                        width: Math.max(root.width, rowLayout.implicitWidth)
                        height: 34
                        color: root.selectedIndex === rowIndex ? "#dbeafe" : (rowIndex % 2 === 0 ? "#ffffff" : "#f8fafc")
                        border.color: "#e5eaf1"

                        RowLayout {
                            id: rowLayout
                            anchors.fill: parent
                            spacing: 0
                            Repeater {
                                model: root.columns
                                delegate: Label {
                                    Layout.preferredWidth: modelData.width || 90
                                    Layout.fillWidth: modelData.fill || false
                                    text: root.rows[row.rowIndex][modelData.key] === undefined ? "" : root.rows[row.rowIndex][modelData.key]
                                    color: "#1f2937"
                                    font.pixelSize: 12
                                    leftPadding: 8
                                    verticalAlignment: Text.AlignVCenter
                                    elide: Text.ElideRight
                                }
                            }
                        }

                        MouseArea {
                            anchors.fill: parent
                            onClicked: {
                                root.selectedIndex = row.rowIndex
                                root.rowSelected(row.rowIndex)
                            }
                            onDoubleClicked: root.editRequested(row.rowIndex)
                        }
                    }
                }
            }
        }
    }
}

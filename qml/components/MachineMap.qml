import QtQuick
import QtQuick.Controls

Item {
    id: root
    property bool compact: width < 760

    Rectangle {
        anchors.fill: parent
        radius: 6
        color: "#f8fafc"
        border.color: "#cbd5e1"
    }

    Repeater {
        model: 60
        delegate: Rectangle {
            x: 28 + index * ((root.width - 56) / 60)
            y: 22
            width: Math.max(4, (root.width - 80) / 76)
            height: 20
            radius: 1
            color: index % 13 === 0 ? "#f59e0b" : "#60a5fa"
        }
    }

    Repeater {
        model: 60
        delegate: Rectangle {
            x: 28 + index * ((root.width - 56) / 60)
            y: root.height - 42
            width: Math.max(4, (root.width - 80) / 76)
            height: 20
            radius: 1
            color: index % 17 === 0 ? "#ef4444" : "#38bdf8"
        }
    }

    Text { text: "后60飞达"; x: 22; y: 4; font.pixelSize: 12; color: "#475569" }
    Text { text: "前60飞达"; x: 22; y: root.height - 18; font.pixelSize: 12; color: "#475569" }

    Rectangle {
        x: root.width * 0.18
        y: root.height * 0.28
        width: root.width * 0.64
        height: root.height * 0.36
        radius: 4
        color: "#e2e8f0"
        border.color: "#94a3b8"

        Row {
            anchors.fill: parent
            anchors.margins: 12
            spacing: 8
            Repeater {
                model: ["进板段", "贴装段", "出板段"]
                delegate: Rectangle {
                    width: (parent.width - 16) / 3
                    height: parent.height
                    radius: 3
                    color: index === 1 ? "#bfdbfe" : "#f1f5f9"
                    border.color: "#64748b"
                    Text {
                        anchors.centerIn: parent
                        text: modelData + "\n三段式轨道"
                        horizontalAlignment: Text.AlignHCenter
                        color: "#1e3a8a"
                        font.pixelSize: root.compact ? 10 : 12
                    }
                }
            }
        }
    }

    Row {
        x: root.width * 0.25
        y: root.height * 0.16
        spacing: Math.max(4, root.width * 0.012)
        Repeater {
            model: 8
            delegate: Rectangle {
                width: Math.max(24, root.width * 0.052)
                height: Math.max(24, root.width * 0.052)
                radius: 4
                color: index === 6 ? "#fef3c7" : "#16a34a"
                border.color: "#166534"
                Text {
                    anchors.centerIn: parent
                    text: "H" + (index + 1)
                    color: index === 6 ? "#92400e" : "white"
                    font.pixelSize: 12
                    font.weight: Font.DemiBold
                }
            }
        }
    }

    Text {
        x: root.width * 0.25
        y: root.height * 0.11
        text: "8贴头"
        color: "#166534"
        font.pixelSize: 13
        font.weight: Font.DemiBold
    }

    Rectangle {
        x: root.width * 0.04
        y: root.height * 0.34
        width: root.width * 0.11
        height: root.height * 0.24
        radius: 4
        color: "#ecfeff"
        border.color: "#0891b2"
        Text { anchors.centerIn: parent; text: "托盘"; color: "#155e75"; font.pixelSize: 13 }
    }

    Repeater {
        model: [
            { label: "飞行相机", px: 0.76, py: 0.18 },
            { label: "Mark相机", px: 0.83, py: 0.42 },
            { label: "大物件相机", px: 0.78, py: 0.68 }
        ]
        delegate: Rectangle {
            x: root.width * modelData.px
            y: root.height * modelData.py
            width: root.width * 0.15
            height: 28
            radius: 14
            color: "#1d4ed8"
            Text {
                anchors.centerIn: parent
                text: modelData.label
                color: "white"
                font.pixelSize: root.compact ? 10 : 12
            }
        }
    }
}

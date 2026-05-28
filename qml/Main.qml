import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import SmtPnpHmi
import "components"
import "pages"

ApplicationWindow {
    id: root
    visible: true
    width: captureWidth > 0 ? captureWidth : Math.max(800, Screen.width * 0.72)
    height: captureHeight > 0 ? captureHeight : Math.max(600, Screen.height * 0.72)
    minimumWidth: 800
    minimumHeight: 600
    title: "SMT Pick And Place HMI"
    color: "#eef2f7"

    property int activePage: 0
    readonly property bool compact: width < 1050
    readonly property bool designFidelityMode: true
    property int captureIndex: 0
    readonly property var args: Qt.application.arguments
    readonly property string captureDir: argumentValue("--capture-dir")
    readonly property int captureWidth: parseInt(argumentValue("--capture-width") || "0")
    readonly property int captureHeight: parseInt(argumentValue("--capture-height") || "0")
    readonly property int captureActionPage: parseInt(argumentValue("--capture-action-page") || "-1")
    readonly property string captureAction: argumentValue("--capture-action")
    readonly property bool selfTestMode: args.indexOf("--self-test") >= 0
    readonly property var pageNames: ["production", "program", "feeders", "heads", "vision", "conveyor", "motion", "maintenance", "logs"]

    AppStore { id: appStore }

    ColumnLayout {
        id: shell
        anchors.fill: parent
        spacing: 0

        TopBar {
            visible: !root.designFidelityMode
            Layout.fillWidth: true
            Layout.preferredHeight: visible ? 56 : 0
            store: appStore
        }

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 0

            NavRail {
                visible: !root.designFidelityMode
                Layout.preferredWidth: root.compact ? 74 : 92
                Layout.fillHeight: true
                currentIndex: root.activePage
                onNavigate: root.activePage = index
            }

            Rectangle {
                Layout.fillWidth: true
                Layout.fillHeight: true
                color: "#e8edf4"

                DesignImagePage {
                    id: designPage
                    visible: root.designFidelityMode
                    anchors.fill: parent
                    store: appStore
                    pageIndex: root.activePage
                    pageName: ["生产", "程序", "飞达", "贴头", "视觉", "轨道", "运动", "维护", "日志"][root.activePage]
                    source: [
                        "qrc:/qt/qml/SmtPnpHmi/qml/assets/design/production.png",
                        "qrc:/qt/qml/SmtPnpHmi/qml/assets/design/program.png",
                        "qrc:/qt/qml/SmtPnpHmi/qml/assets/design/feeders.png",
                        "qrc:/qt/qml/SmtPnpHmi/qml/assets/design/heads.png",
                        "qrc:/qt/qml/SmtPnpHmi/qml/assets/design/vision.png",
                        "qrc:/qt/qml/SmtPnpHmi/qml/assets/design/conveyor.png",
                        "qrc:/qt/qml/SmtPnpHmi/qml/assets/design/motion.png",
                        "qrc:/qt/qml/SmtPnpHmi/qml/assets/design/maintenance.png",
                        "qrc:/qt/qml/SmtPnpHmi/qml/assets/design/logs.png"
                    ][root.activePage]
                    onNavigate: root.activePage = index
                }

                Loader {
                    id: pageLoader
                    visible: !root.designFidelityMode
                    anchors.fill: parent
                    anchors.margins: root.compact ? 8 : 14
                    sourceComponent: [
                        productionPage,
                        programPage,
                        feedersPage,
                        headsPage,
                        visionPage,
                        conveyorPage,
                        motionPage,
                        maintenancePage,
                        logsPage
                    ][root.activePage]
                    onLoaded: {
                        item.store = appStore
                        item.compact = Qt.binding(function() { return root.compact })
                    }
                }
            }
        }
    }

    Component { id: productionPage; ProductionPage {} }
    Component { id: programPage; ProgramPage {} }
    Component { id: feedersPage; FeedersPage {} }
    Component { id: headsPage; HeadsPage {} }
    Component { id: visionPage; VisionPage {} }
    Component { id: conveyorPage; ConveyorPage {} }
    Component { id: motionPage; MotionPage {} }
    Component { id: maintenancePage; MaintenancePage {} }
    Component { id: logsPage; LogsPage {} }

    function argumentValue(name) {
        var idx = args.indexOf(name)
        return idx >= 0 && idx + 1 < args.length ? args[idx + 1] : ""
    }

    function captureNextPage() {
        if (captureDir.length === 0)
            return
        if (captureIndex >= pageNames.length) {
            Qt.quit()
            return
        }
        activePage = captureIndex
        captureTimer.restart()
    }

    Timer {
        id: captureTimer
        interval: 350
        repeat: false
        onTriggered: {
            if (root.captureActionPage === root.captureIndex && root.captureAction.length > 0)
                designPage.handleAction(root.captureAction)
            shell.grabToImage(function(result) {
                var suffix = root.captureActionPage === root.captureIndex && root.captureAction.length > 0 ? "-" + root.captureAction : ""
                var path = captureDir + "/" + pageNames[captureIndex] + suffix + "-" + root.width + "x" + root.height + ".png"
                result.saveToFile(path)
                captureIndex += 1
                captureNextPage()
            })
        }
    }

    Timer {
        id: selfTestTimer
        interval: 0
        repeat: false
        onTriggered: {
            var ok = appStore.runCrudSelfTest()
            ok = machineBackend.runSelfTest() && ok
            if (!ok)
                throw new Error("SMT PNP HMI CRUD self-test failed")
            console.log("SMT PNP HMI CRUD/backend self-test passed")
            Qt.quit()
        }
    }

    Component.onCompleted: {
        if (root.selfTestMode)
            selfTestTimer.start()
        else if (captureDir.length > 0)
            captureNextPage()
    }
}

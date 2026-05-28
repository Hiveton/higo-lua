#include <QGuiApplication>
#include <QQmlContext>
#include <QQmlApplicationEngine>
#include <QQuickStyle>

#include "machine_backend.h"

int main(int argc, char *argv[])
{
    QGuiApplication app(argc, argv);
    QGuiApplication::setOrganizationName("Hiveton");
    QGuiApplication::setApplicationName("SMT Pick And Place HMI");
    QQuickStyle::setStyle("Fusion");

    QQmlApplicationEngine engine;
    MachineBackend machineBackend;
    engine.rootContext()->setContextProperty(QStringLiteral("machineBackend"), &machineBackend);
    QObject::connect(
        &engine,
        &QQmlApplicationEngine::objectCreationFailed,
        &app,
        []() { QCoreApplication::exit(-1); },
        Qt::QueuedConnection);
    engine.loadFromModule("SmtPnpHmi", "Main");

    return app.exec();
}

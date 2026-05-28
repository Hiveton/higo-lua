#pragma once

#include <QObject>
#include <QString>
#include <QVariantMap>

class MachineBackend : public QObject
{
    Q_OBJECT

public:
    explicit MachineBackend(QObject *parent = nullptr);

    Q_INVOKABLE bool executeCommand(const QString &module, const QString &command);
    Q_INVOKABLE bool homeAll();
    Q_INVOKABLE bool enableServo(bool enabled);
    Q_INVOKABLE bool jog(const QString &axis, double distance, double speed);
    Q_INVOKABLE bool bindFeeder(const QString &side, const QString &slot, const QString &barcode);
    Q_INVOKABLE QVariantMap capture(const QString &cameraName);
    Q_INVOKABLE bool setRailWidth(double widthMm);
    Q_INVOKABLE bool appendAlarm(const QVariantMap &alarm);
    Q_INVOKABLE bool runSelfTest();

signals:
    void eventLogged(const QString &module, const QString &message);

private:
    void log(const QString &module, const QString &message);
    QString m_lastCommand;
};

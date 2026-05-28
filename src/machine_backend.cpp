#include "machine_backend.h"

#include <QDateTime>

MachineBackend::MachineBackend(QObject *parent)
    : QObject(parent)
{
}

bool MachineBackend::executeCommand(const QString &module, const QString &command)
{
    if (module.trimmed().isEmpty() || command.trimmed().isEmpty())
        return false;
    m_lastCommand = module + ":" + command;
    log(module, "command=" + command);
    return true;
}

bool MachineBackend::homeAll()
{
    m_lastCommand = "MotionService:homeAll";
    log("MotionService", "homeAll");
    return true;
}

bool MachineBackend::enableServo(bool enabled)
{
    m_lastCommand = QString("MotionService:enableServo:%1").arg(enabled);
    log("MotionService", enabled ? "servo enabled" : "servo disabled");
    return true;
}

bool MachineBackend::jog(const QString &axis, double distance, double speed)
{
    if (axis.trimmed().isEmpty() || speed < 0)
        return false;
    m_lastCommand = QString("MotionService:jog:%1:%2:%3").arg(axis).arg(distance).arg(speed);
    log("MotionService", m_lastCommand);
    return true;
}

bool MachineBackend::bindFeeder(const QString &side, const QString &slot, const QString &barcode)
{
    if (side.trimmed().isEmpty() || slot.trimmed().isEmpty() || barcode.trimmed().isEmpty())
        return false;
    m_lastCommand = QString("FeederService:bind:%1:%2:%3").arg(side, slot, barcode);
    log("FeederService", m_lastCommand);
    return true;
}

QVariantMap MachineBackend::capture(const QString &cameraName)
{
    QVariantMap result;
    if (cameraName.trimmed().isEmpty()) {
        result.insert("ok", false);
        result.insert("error", "cameraName is empty");
        return result;
    }

    m_lastCommand = "VisionService:capture:" + cameraName;
    result.insert("ok", true);
    result.insert("camera", cameraName);
    result.insert("timestamp", QDateTime::currentDateTimeUtc().toString(Qt::ISODateWithMs));
    result.insert("result", "OK");
    log("VisionService", m_lastCommand);
    return result;
}

bool MachineBackend::setRailWidth(double widthMm)
{
    if (widthMm <= 0)
        return false;
    m_lastCommand = QString("ConveyorService:setRailWidth:%1").arg(widthMm);
    log("ConveyorService", m_lastCommand);
    return true;
}

bool MachineBackend::appendAlarm(const QVariantMap &alarm)
{
    if (!alarm.contains("code"))
        return false;
    m_lastCommand = "TraceService:appendAlarm:" + alarm.value("code").toString();
    log("TraceService", m_lastCommand);
    return true;
}

bool MachineBackend::runSelfTest()
{
    bool ok = true;
    ok = executeCommand("Production", "启动") && ok;
    ok = homeAll() && ok;
    ok = enableServo(true) && ok;
    ok = jog("X", 1.0, 10.0) && ok;
    ok = bindFeeder("前", "01", "FDR-SELFTEST") && ok;
    ok = capture("飞行相机").value("ok").toBool() && ok;
    ok = setRailWidth(142.0) && ok;
    QVariantMap alarm;
    alarm.insert("code", "SELFTEST");
    alarm.insert("module", "TraceService");
    alarm.insert("desc", "backend self-test");
    ok = appendAlarm(alarm) && ok;
    return ok;
}

void MachineBackend::log(const QString &module, const QString &message)
{
    emit eventLogged(module, message);
}

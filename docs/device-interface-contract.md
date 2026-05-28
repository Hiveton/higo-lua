# 贴片机上位机前端接口契约

本文定义当前 Qt/QML 前端与后续 C++/Qt 后端、PLC/运动控制、相机 SDK、IO 和数据库服务的接口边界。当前版本使用 `AppStore.qml` 的本地快照模型模拟业务数据，并通过 `src/machine_backend.*` 暴露 `machineBackend` C++/Qt 桥接对象。后续接入真实设备时应保持字段结构和 QML 调用边界稳定。

## 数据域

| 域 | 前端集合 | 关键字段 | 说明 |
| --- | --- | --- | --- |
| 生产状态 | machineState/currentProgram/boardCount/cph/cycleTime/yieldRate | 当前程序、生产计数、贴装速度、节拍、良率、设备状态 | 顶部状态栏和生产页使用 |
| 程序 | programRows | 位号、物料、来源、贴头、吸嘴、X/Y、角度、视觉策略、状态 | 坐标导入、路径优化、模拟运行 |
| 飞达 | feeders | 面、槽位、物料、封装、间距、数量、贴头、吸嘴、状态 | 前 60 / 后 60 飞达栈 |
| 贴头 | heads | 贴头编号、吸嘴、状态、真空、Z、角度、成功率 | 8 贴头状态与校准 |
| 视觉 | cameras | 相机名、曝光、增益、光源、模式、偏移、状态 | 飞行相机、Mark 相机、大物件相机 |
| 轨道 | rails | 区域、PCB、宽度、速度、传感器、状态 | 三段式轨道 |
| 托盘 | trays | 托盘编号、物料、穴位、已用、状态 | 大物件供料 |
| 维护 | maintenance | 项目、模块、周期、下次时间、状态 | 维护计划 |
| 日志 | alarms | 时间、等级、代码、模块、描述、处理建议、状态 | 报警、操作、追溯 |

## 后端服务建议

当前 C++ 桥接对象提供以下可调用方法：

```text
machineBackend.executeCommand(module, command)
machineBackend.homeAll()
machineBackend.enableServo(enabled)
machineBackend.jog(axis, distance, speed)
machineBackend.bindFeeder(side, slot, barcode)
machineBackend.capture(cameraName)
machineBackend.setRailWidth(widthMm)
machineBackend.appendAlarm(alarmRecord)
machineBackend.runSelfTest()
```

这些方法现在返回模拟结果，并可作为真实服务接入的替换点。

### MotionService

- `homeAll()`
- `enableServo(enabled)`
- `jog(axis, distance, speed)`
- `moveTo(axisPositions)`
- `readAxisSnapshot()`
- `saveCalibration(calibrationRecord)`

### FeederService

- `bindFeeder(slot, side, barcode)`
- `updateFeeder(feederRecord)`
- `removeFeeder(slot, side)`
- `calibrateFeeder(slot, side)`
- `readFeederMap()`

### VisionService

- `capture(cameraName)`
- `autoExpose(cameraName)`
- `calibrateCamera(cameraName)`
- `testRecognition(strategyRecord)`
- `saveVisionStrategy(strategyRecord)`

### ConveyorService

- `setRailWidth(widthMm)`
- `infeed()`
- `outfeed()`
- `clamp(enabled)`
- `readConveyorSnapshot()`

### TrayService

- `updateTray(trayRecord)`
- `calibrateTray(trayId)`
- `readTrayMap()`

### TraceService

- `appendAlarm(alarmRecord)`
- `ackAlarm(code)`
- `exportTrace(filter)`
- `queryProductionRecords(filter)`

## 快照接口

当前前端已经提供以下函数，可作为 C++/Qt 后端桥接的最小接口：

```text
createSnapshot()
applySnapshot(snapshot)
persistSnapshot()
loadPersistedSnapshot()
clearPersistedSnapshot()
exportSnapshotJson()
importSnapshotJson(jsonText)
runCrudSelfTest()
```

## 交付验证

`tests/run_full_verification.py` 会生成 `artifacts/verification-report.json`。验收时至少检查：

- `status` 为 `passed`
- `self_test` 为 `passed`
- `capture_sets` 覆盖 `800x600`、`1920x1080`、`3840x2160`
- 每个分辨率都有 9 张页面截图
- `dialog_captures` 覆盖飞达、视觉、日志新增弹层

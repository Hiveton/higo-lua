# SMT Pick And Place HMI

Qt Quick/QML 贴片机上位机前端原型，面向 8 贴头、前 60/后 60 飞达栈、托盘、三段式轨道、飞行相机、Mark 相机、大物件相机的完整操作界面。

当前主界面启用严格设计稿还原模式：`qml/Main.qml` 使用 `DesignImagePage` 直接渲染 `qml/assets/design/*.png` 原设计图，并在导航和操作按钮位置覆盖透明交互热区。这样先保证视觉一比一，后续再逐块替换为原子组件。

`src/machine_backend.*` 提供 C++/Qt 设备服务桥接对象 `machineBackend`，当前是可替换的模拟实现。QML 的保存、删除、启动、暂停、停止等动作会调用该桥接层，后续真实设备接入时可在这里替换运动控制、飞达、视觉、轨道、托盘和追溯服务。

## 页面

- 生产：设备总览、8 贴头、前后飞达、托盘、三段式轨道、相机位置、启动/暂停/停止/复位/原点/DryRun。
- 程序：PCB 坐标编辑、贴装表、元件来源、贴头/吸嘴/视觉策略，支持新增、编辑、删除、保存。
- 飞达：前 60/后 60 栈位图、飞达清单、托盘物料、扫码绑定、校准，支持新增、编辑、删除、保存。
- 贴头：8 贴头状态、吸嘴库、真空、高度、角度、成功率和维护动作。
- 视觉：飞行相机、Mark 相机、大物件相机预览和视觉策略参数。
- 轨道：三段式轨道、传感器、调宽、进板、出板、托盘校准。
- 运动：坐标总览、点动、轴状态、伺服和校准动作。
- 维护：I/O 诊断、维护清单、自检、备份、工程师模式。
- 日志：报警、操作记录、生产追溯，支持新增报警和确认报警。

## 构建

需要 Qt 6.5+，本机已验证 Qt 6.11。

```bash
cmake -S . -B build
cmake --build build -j4
./build/smt_pnp_hmi
```

## 校验

```bash
python3 tests/validate_project.py
```

完整回归会执行源码结构校验、CMake 构建、800x600/1920x1080/3840x2160 九页截图，以及飞达/视觉/日志字段弹层截图，并检查 PNG 数量和尺寸：
同时会运行 `--self-test`，实际执行新增、修改、删除、导出、导入、保存、恢复快照等数据链路，并验证 C++ `machineBackend` 桥接接口。

```bash
python3 tests/run_full_verification.py
```

完整回归成功后会写入机器可读验收报告：

```text
artifacts/verification-report.json
```

生成可交付目录和 zip 包：

```bash
python3 tools/package_release.py
```

输出内容：

```text
artifacts/release/smt-pnp-hmi-deliverable/
artifacts/release/smt-pnp-hmi-deliverable.zip
artifacts/release/smt-pnp-hmi-deliverable/manifest.json
```

交付包包含完整源码、Qt/QML 设计图还原资源、原设计图切图、800x600/1920x1080/3840x2160 九页测试截图、弹层交互截图、验收报告、接口边界文档和已构建的本机可执行文件。

需求到证据的验收矩阵：

```text
docs/acceptance-matrix.md
```

单独运行数据链路自检：

```bash
QT_QPA_PLATFORM=offscreen ./build/smt_pnp_hmi --self-test
```

## 本地持久化

`qml/AppStore.qml` 使用 Qt `Settings` 保存完整前端数据快照。程序、飞达、贴头、视觉相机、轨道、托盘、维护和日志数据在前端增删改后会自动保存，应用重启后优先恢复本地快照。

可用函数：

```text
createSnapshot()
applySnapshot(snapshot)
persistSnapshot()
loadPersistedSnapshot()
clearPersistedSnapshot()
exportSnapshotJson()
importSnapshotJson(jsonText)
```

后续如果要接数据库或设备控制服务，可以把 `exportSnapshotJson()` 的数据结构作为前后端接口草案。

## 设计图与切图

原设计图位于：

```text
qml/assets/design/
```

已从每个原设计图切出 topbar、nav、workspace、right_panel、bottom_status 五类资产：

```bash
python3 tools/cut_design_assets.py
```

输出目录：

```text
qml/assets/cuts/
```

## 自动截图

应用内置测试截图模式，不依赖系统桌面截屏权限：

```bash
QT_QPA_PLATFORM=offscreen ./build/smt_pnp_hmi \
  --capture-dir artifacts/captures-800 \
  --capture-width 800 \
  --capture-height 600
```

同理可生成 1920x1080 或 3840x2160 截图。

也可以在指定页面自动打开交互弹层后截图，例如飞达页新增弹层：

```bash
QT_QPA_PLATFORM=offscreen ./build/smt_pnp_hmi \
  --capture-dir artifacts/captures-dialog \
  --capture-width 1920 \
  --capture-height 1080 \
  --capture-action-page 2 \
  --capture-action add
```

## 当前边界

当前版本是可运行前端与 Qt 本地持久化数据模型。真实运动控制、相机 SDK、PLC/IO、数据库和权限系统尚未接入，后续应把 `qml/AppStore.qml` 中的数据快照模型替换或同步到 C++/Qt 后端模型、SQLite 数据库或设备服务接口。
后端接口边界见 `docs/device-interface-contract.md`。

# 贴片机上位机前端验收矩阵

本文把用户目标拆成可检查的验收项，并指向当前工程中的权威证据。验收时以源码、自动化验证报告、截图和交付包清单为准。

## 验收范围

| 目标要求 | 当前实现 | 验收证据 |
| --- | --- | --- |
| Qt 贴片机上位机前端系统 | Qt Quick/QML 应用，C++ `MachineBackend` 桥接对象提供设备服务替换点 | `CMakeLists.txt`、`src/main.cpp`、`src/machine_backend.*` |
| 严格根据设计稿还原 | `DesignImagePage` 直接渲染原设计图背景，并覆盖透明导航和操作热区 | `qml/components/DesignImagePage.qml`、`qml/assets/design/*.png`、`artifacts/captures-*/*.png` |
| 从原设计图切图 | 已按 topbar、nav、workspace、right_panel、bottom_status 切出资产 | `tools/cut_design_assets.py`、`qml/assets/cuts/*.png` |
| 8 贴头 | 贴头页和生产页展示 8 贴头状态，数据模型提供 8 个 head 记录 | `qml/AppStore.qml`、`qml/assets/design/heads.png`、`artifacts/captures-1920/heads-1920x1080.png` |
| 前 60 / 后 60 飞达 | 飞达页覆盖前后飞达栈位、飞达清单、扫码绑定字段 | `qml/AppStore.qml`、`qml/assets/design/feeders.png`、`artifacts/captures-1920/feeders-1920x1080.png` |
| 托盘支持 | 托盘数据域、轨道页托盘校准、生产页托盘状态 | `qml/AppStore.qml`、`qml/assets/design/conveyor.png`、`docs/device-interface-contract.md` |
| 三段式轨道 | 进板段、贴装段、出板段三段状态和轨道宽度字段 | `qml/components/DesignImagePage.qml`、`artifacts/captures-1920/conveyor-1920x1080.png` |
| 飞行相机 / Mark 相机 / 大物件相机 | 视觉页提供三类相机参数和新增/编辑/保存交互 | `qml/AppStore.qml`、`qml/assets/design/vision.png`、`artifacts/captures-dialog/vision-add-1920x1080.png` |
| 完整控制页面 | 生产、程序、飞达、贴头、视觉、轨道、运动、维护、日志 9 个页面 | `qml/Main.qml`、`tests/run_full_verification.py`、`artifacts/verification-report.json` |
| 完整字段 | 程序、飞达、贴头、视觉、轨道、运动、维护、日志弹层定义业务字段 | `qml/components/DesignImagePage.qml`、`tests/validate_project.py` |
| 增删改查 | `AppStore` 提供 add/update/delete/upsert/export/import/persist/load，自检实际执行数据链路 | `qml/AppStore.qml`、`QT_QPA_PLATFORM=offscreen ./build/smt_pnp_hmi --self-test` |
| 设备命令交互 | 保存、删除、启动、暂停、停止等动作调用 `machineBackend.executeCommand()` | `qml/components/DesignImagePage.qml`、`src/machine_backend.*` |
| 800x600 最小分辨率 | 自动截图生成 9 页 800x600，并检查 PNG 尺寸 | `artifacts/captures-800/*.png`、`artifacts/verification-report.json` |
| 4K 最大分辨率 | 自动截图生成 9 页 3840x2160，并检查 PNG 尺寸 | `artifacts/captures-4k/*.png`、`artifacts/verification-report.json` |
| 测试截图交付 | 交付包包含 27 张主页面截图和弹层交互截图 | `artifacts/release/smt-pnp-hmi-deliverable/manifest.json` |
| 代码交付 | 交付包包含源码、文档、测试、工具、设计资源、截图和本机可执行文件 | `tools/package_release.py`、`artifacts/release/smt-pnp-hmi-deliverable.zip` |

## 自动化验收命令

```bash
python3 tests/validate_project.py
python3 tests/run_full_verification.py
python3 tools/package_release.py
```

## 必查产物

```text
artifacts/verification-report.json
artifacts/release/smt-pnp-hmi-deliverable/manifest.json
artifacts/release/smt-pnp-hmi-deliverable.zip
artifacts/captures-800/
artifacts/captures-1920/
artifacts/captures-4k/
artifacts/captures-dialog/
```

## 当前边界

当前工程已经满足可交付前端、交互、字段、CRUD、本地持久化、Qt 后端桥接和测试截图要求。真实运动控制器、相机 SDK、PLC/IO、数据库和权限系统属于设备集成阶段，不在当前前端交付包内；后续接入时应遵循 `docs/device-interface-contract.md` 的服务边界。

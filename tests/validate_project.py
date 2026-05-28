#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

required_files = [
    "CMakeLists.txt",
    "src/main.cpp",
    "src/machine_backend.h",
    "src/machine_backend.cpp",
    "qml/Main.qml",
    "qml/AppStore.qml",
    "qml/components/TopBar.qml",
    "qml/components/NavRail.qml",
    "qml/components/DataTable.qml",
    "qml/components/DesignImagePage.qml",
    "qml/pages/ProductionPage.qml",
    "qml/pages/ProgramPage.qml",
    "qml/pages/FeedersPage.qml",
    "qml/pages/HeadsPage.qml",
    "qml/pages/VisionPage.qml",
    "qml/pages/ConveyorPage.qml",
    "qml/pages/MotionPage.qml",
    "qml/pages/MaintenancePage.qml",
    "qml/pages/LogsPage.qml",
    "tests/run_full_verification.py",
    "tools/package_release.py",
    "docs/acceptance-matrix.md",
    "docs/device-interface-contract.md",
]

missing = [name for name in required_files if not (ROOT / name).exists()]
assert not missing, f"missing files: {missing}"

for page in ["production", "program", "feeders", "heads", "vision", "conveyor", "motion", "maintenance", "logs"]:
    assert (ROOT / f"qml/assets/design/{page}.png").exists(), f"missing design asset: {page}.png"

content = "\n".join(path.read_text(encoding="utf-8") for path in (ROOT / "qml").rglob("*.qml"))

for label in [
    "8贴头",
    "前60飞达",
    "后60飞达",
    "托盘",
    "三段式轨道",
    "飞行相机",
    "Mark相机",
    "大物件相机",
    "新增",
    "编辑",
    "删除",
    "保存",
]:
    assert label in content, f"missing UI label: {label}"

for fn in [
    "createSnapshot",
    "applySnapshot",
    "persistSnapshot",
    "loadPersistedSnapshot",
    "clearPersistedSnapshot",
    "exportSnapshotJson",
    "importSnapshotJson",
    "runCrudSelfTest",
    "addFeeder",
    "updateFeeder",
    "deleteFeeder",
    "addProgramRow",
    "updateProgramRow",
    "deleteProgramRow",
    "upsertHead",
    "upsertCamera",
    "upsertRail",
    "upsertTray",
    "upsertMaintenance",
    "addAlarm",
    "clearAlarm",
]:
    assert fn in content, f"missing CRUD/control function: {fn}"

app_store = (ROOT / "qml/AppStore.qml").read_text(encoding="utf-8")
assert "import QtCore" in app_store, "AppStore must import QtCore for Settings persistence"
assert "Settings" in app_store, "AppStore must use Qt Settings persistence"
for persisted_key in [
    "programRows",
    "feeders",
    "heads",
    "cameras",
    "rails",
    "trays",
    "alarms",
    "maintenance",
]:
    assert persisted_key in app_store, f"missing persisted collection: {persisted_key}"

design_page = (ROOT / "qml/components/DesignImagePage.qml").read_text(encoding="utf-8")
for schema_name in [
    "productionFields",
    "programFields",
    "feederFields",
    "headFields",
    "visionFields",
    "conveyorFields",
    "motionFields",
    "maintenanceFields",
    "logFields",
]:
    assert schema_name in design_page, f"missing page schema: {schema_name}"

for field in [
    "当前程序", "生产计数", "贴装速度", "位号", "X坐标", "Y坐标", "飞达槽位",
    "前60飞达", "后60飞达", "托盘编号", "贴头编号", "吸嘴型号", "真空值",
    "飞行相机", "Mark相机", "大物件相机", "曝光", "光源", "进板段",
    "贴装段", "出板段", "轨道宽度", "X轴", "Y轴", "Z轴", "维护项目",
    "报警代码", "处理建议"
]:
    assert field in design_page, f"missing required field label: {field}"

for action in ["createRecord", "updateRecord", "deleteRecord", "saveRecord", "selectedRows"]:
    assert action in design_page, f"missing design interaction helper: {action}"

main = (ROOT / "qml/Main.qml").read_text(encoding="utf-8")
assert "minimumWidth: 800" in main
assert "minimumHeight: 600" in main
assert "captureWidth > 0 ? captureWidth : Math.max(800" in main
assert "captureHeight > 0 ? captureHeight : Math.max(600" in main
assert "--capture-dir" in main
assert "--capture-action-page" in main
assert "--capture-action" in main
assert "--self-test" in main
assert "runCrudSelfTest" in main
assert "DesignImagePage" in main
assert "designFidelityMode: true" in main

readme = (ROOT / "README.md").read_text(encoding="utf-8")
assert "本地持久化" in readme
assert "exportSnapshotJson" in readme
assert "run_full_verification.py" in readme
assert "verification-report.json" in readme
assert "package_release.py" in readme
assert "manifest.json" in readme
assert "acceptance-matrix.md" in readme

contract = (ROOT / "docs/device-interface-contract.md").read_text(encoding="utf-8")
for contract_term in ["MotionService", "FeederService", "VisionService", "ConveyorService", "TrayService", "TraceService"]:
    assert contract_term in contract, f"missing interface contract term: {contract_term}"

acceptance = (ROOT / "docs/acceptance-matrix.md").read_text(encoding="utf-8")
for acceptance_term in [
    "Qt 贴片机上位机前端系统",
    "严格根据设计稿还原",
    "从原设计图切图",
    "8 贴头",
    "前 60 / 后 60 飞达",
    "托盘支持",
    "三段式轨道",
    "飞行相机 / Mark 相机 / 大物件相机",
    "增删改查",
    "800x600",
    "4K",
    "manifest.json",
]:
    assert acceptance_term in acceptance, f"missing acceptance matrix term: {acceptance_term}"

full_verification = (ROOT / "tests/run_full_verification.py").read_text(encoding="utf-8")
assert "verification-report.json" in full_verification
assert '"status": "passed"' in full_verification or 'report["status"] = "passed"' in full_verification

package_release = (ROOT / "tools/package_release.py").read_text(encoding="utf-8")
for package_term in [
    "smt-pnp-hmi-deliverable",
    "manifest.json",
    "zipfile",
    "verification-report.json",
    "captures-4k",
    "acceptance-matrix.md",
]:
    assert package_term in package_release, f"missing package release term: {package_term}"

main_cpp = (ROOT / "src/main.cpp").read_text(encoding="utf-8")
assert "MachineBackend" in main_cpp
assert "machineBackend" in main_cpp
assert "setContextProperty" in main_cpp

backend = (ROOT / "src/machine_backend.h").read_text(encoding="utf-8") + "\n" + (ROOT / "src/machine_backend.cpp").read_text(encoding="utf-8")
for backend_symbol in [
    "executeCommand",
    "homeAll",
    "enableServo",
    "jog",
    "bindFeeder",
    "capture",
    "setRailWidth",
    "appendAlarm",
    "runSelfTest",
]:
    assert backend_symbol in backend, f"missing backend symbol: {backend_symbol}"

assert "machineBackend.runSelfTest" in main
assert "machineBackend.executeCommand" in design_page

print("smt-pnp-hmi project validation passed")

cmake_minimum_required(VERSION 3.21)

project(SmtPnpHmi VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(Qt6 REQUIRED COMPONENTS Quick QuickControls2)

qt_standard_project_setup(REQUIRES 6.5)

qt_add_executable(smt_pnp_hmi
    src/main.cpp
    src/machine_backend.cpp
    src/machine_backend.h
)

qt_add_qml_module(smt_pnp_hmi
    URI SmtPnpHmi
    VERSION 1.0
    QML_FILES
        qml/Main.qml
        qml/AppStore.qml
        qml/components/DataTable.qml
        qml/components/DesignImagePage.qml
        qml/components/MachineMap.qml
        qml/components/NavRail.qml
        qml/components/Section.qml
        qml/components/TopBar.qml
        qml/pages/ConveyorPage.qml
        qml/pages/FeedersPage.qml
        qml/pages/HeadsPage.qml
        qml/pages/LogsPage.qml
        qml/pages/MaintenancePage.qml
        qml/pages/MotionPage.qml
        qml/pages/ProductionPage.qml
        qml/pages/ProgramPage.qml
        qml/pages/VisionPage.qml
    RESOURCES
        qml/assets/design/conveyor.png
        qml/assets/design/feeders.png
        qml/assets/design/heads.png
        qml/assets/design/logs.png
        qml/assets/design/maintenance.png
        qml/assets/design/motion.png
        qml/assets/design/production.png
        qml/assets/design/program.png
        qml/assets/design/reference-overview.png
        qml/assets/design/vision.png
)

target_link_libraries(smt_pnp_hmi
    PRIVATE
        Qt6::Quick
        Qt6::QuickControls2
)

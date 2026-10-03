import bb.cascades 1.4

Page {
    property variant c: nav.chip

    titleBar: TitleBar { title: qsTr("eUICC info") }

    actions: [
        ActionItem {
            title: qsTr("Copy EID")
            imageSource: "asset:///images/copy.png"
            ActionBar.placement: ActionBarPlacement.Signature
            onTriggered: bridge.copy(c.eid)
        }
    ]

    ScrollView {
        Container {
            bottomPadding: ui.du(2)
            Field { label: qsTr("EID"); value: c.eid }
            Field { label: qsTr("Default SM-DP+"); value: c.defaultSmdp }
            Field { label: qsTr("Root SM-DS"); value: c.rootSmds }
            Field { label: qsTr("SGP.22 version"); value: c.svn }
            Field { label: qsTr("Profile package version"); value: c.profileVersion }
            Field { label: qsTr("Firmware"); value: c.firmwareVersion }
            Field {
                label: qsTr("Free memory")
                value: c.freeNvm != undefined
                       ? qsTr("%1 KB storage, %2 KB RAM").arg(Math.round(c.freeNvm / 1024)).arg(Math.round(c.freeVm / 1024))
                       : ""
            }
            Field { label: qsTr("SAS accreditation"); value: c.sas }
            Field { label: qsTr("Category"); value: c.category }
            Field { label: qsTr("Capabilities"); value: c.rspCapabilities ? c.rspCapabilities.join(", ") : "" }
            Field { label: qsTr("CI public key IDs"); value: c.ciPkids ? c.ciPkids.join("\n") : "" }
        }
    }
}

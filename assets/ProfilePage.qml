import bb.cascades 1.4
import bb.system 1.2

Page {
    id: page
    objectName: "profilePage"

    property string iccid
    // Re-evaluated whenever the profile list is refreshed.
    property variant p: find(nav.profiles, iccid)
    property string title: p.nickname || p.name || p.provider || iccid

    function find(list, id) {
        for (var i = 0; i < list.length; i++)
            if (list[i].iccid == id)
                return list[i];
        return {};
    }

    titleBar: TitleBar { title: page.title }

    actions: [
        ActionItem {
            title: p.enabled ? qsTr("Disable") : qsTr("Enable")
            imageSource: p.enabled ? "asset:///images/disable.png" : "asset:///images/enable.png"
            ActionBar.placement: ActionBarPlacement.Signature
            enabled: !nav.busy && p.iccid != undefined
            onTriggered: {
                switchDialog.iccid = iccid;
                switchDialog.enable = !p.enabled;
                switchDialog.title = p.enabled ? qsTr("Disable “%1”?").arg(page.title) : qsTr("Switch to “%1”?").arg(page.title);
                switchDialog.body = p.enabled
                    ? qsTr("The phone has no mobile service until another profile is enabled.")
                    : qsTr("The current profile is disabled and the phone loses service for a moment while it switches.");
                switchDialog.confirmButton.label = p.enabled ? qsTr("Disable") : qsTr("Enable");
                switchDialog.show();
            }
        },
        ActionItem {
            title: qsTr("Rename")
            imageSource: "asset:///images/rename.png"
            ActionBar.placement: ActionBarPlacement.OnBar
            enabled: !nav.busy && p.iccid != undefined
            onTriggered: {
                renamePrompt.inputField.defaultText = p.nickname || "";
                renamePrompt.show();
            }
        },
        ActionItem {
            title: qsTr("Copy ICCID")
            imageSource: "asset:///images/copy.png"
            onTriggered: bridge.copy(iccid)
        },
        DeleteActionItem {
            title: qsTr("Delete")
            enabled: !nav.busy && p.iccid != undefined && !p.enabled
            onTriggered: deleteDialog.show()
        }
    ]

    ScrollView {
        Container {
            layout: StackLayout {}

            BusyBar {}

            Container {
                leftPadding: ui.du(2); rightPadding: ui.du(2); topPadding: ui.du(2)
                Label {
                    text: page.title
                    textStyle.base: SystemDefaults.TextStyles.TitleText
                    multiline: true
                }
                Label {
                    text: p.enabled ? qsTr("Enabled") : qsTr("Disabled")
                    textStyle.color: p.enabled ? ui.palette.primary : ui.palette.secondaryTextOnPlain
                }
            }

            Label {
                visible: p.enabled == true
                leftPadding: ui.du(2); rightPadding: ui.du(2)
                text: qsTr("Disable this profile before deleting it.")
                multiline: true
                textStyle.base: SystemDefaults.TextStyles.SubtitleText
                textStyle.color: ui.palette.secondaryTextOnPlain
            }

            Field { label: qsTr("Provider"); value: p.provider }
            Field { label: qsTr("Profile name"); value: p.name }
            Field { label: qsTr("Nickname"); value: p.nickname }
            Field { label: qsTr("ICCID"); value: p.iccid }
            Field { label: qsTr("Class"); value: p.profileClass }
            Field { label: qsTr("ISD-P AID"); value: p.isdpAid }
        }
    }

    attachedObjects: [
        SwitchDialog { id: switchDialog },
        SystemPrompt {
            id: renamePrompt
            title: qsTr("Nickname")
            body: qsTr("Leave empty to remove the nickname.")
            inputField.emptyText: qsTr("Nickname")
            inputField.maximumLength: 64
            confirmButton.label: qsTr("Save")
            cancelButton.label: qsTr("Cancel")
            onFinished: {
                if (value == SystemUiResult.ConfirmButtonSelection)
                    nav.setNickname(iccid, inputFieldTextEntry());
            }
        },
        SystemDialog {
            id: deleteDialog
            title: qsTr("Delete “%1”?").arg(page.title)
            body: qsTr("The profile is erased from the eUICC. Most operators do not let you download the same profile again.")
            confirmButton.label: qsTr("Delete")
            cancelButton.label: qsTr("Cancel")
            onFinished: {
                if (value == SystemUiResult.ConfirmButtonSelection)
                    nav.deleteProfile(iccid);
            }
        }
    ]
}

import bb.cascades 1.4

Page {
    id: page

    titleBar: TitleBar { title: qsTr("eSIM profiles") }

    actions: [
        ActionItem {
            title: qsTr("Add profile")
            imageSource: "asset:///images/add.png"
            ActionBar.placement: ActionBarPlacement.Signature
            enabled: nav.conn == "ready" && !nav.busy
            onTriggered: nav.openPage(downloadPageDef)
        },
        ActionItem {
            title: qsTr("Refresh")
            imageSource: "asset:///images/refresh.png"
            ActionBar.placement: ActionBarPlacement.OnBar
            enabled: !nav.busy
            onTriggered: nav.refresh()
        },
        ActionItem {
            title: nav.notifications.length > 0 ? qsTr("Notifications (%1)").arg(nav.notifications.length)
                                                 : qsTr("Notifications")
            imageSource: "asset:///images/notifications.png"
            enabled: nav.conn == "ready"
            onTriggered: nav.openPage(notificationsPageDef)
        },
        ActionItem {
            title: qsTr("eUICC info")
            imageSource: "asset:///images/info.png"
            enabled: nav.chip.eid != undefined
            onTriggered: nav.openPage(infoPageDef)
        }
    ]

    Container {
        layout: StackLayout {}

        BusyBar {}

        // Not connected: explain why and offer a way out.
        Container {
            visible: nav.conn != "ready" && nav.conn != "connecting"
            leftPadding: ui.du(2); rightPadding: ui.du(2); topPadding: ui.du(4)

            Label {
                text: nav.conn == "noAccess" ? qsTr("No access to the SIM")
                    : nav.conn == "notReady" ? qsTr("The SIM is not ready")
                    : nav.conn == "noEuicc" ? qsTr("No eUICC found")
                    : qsTr("Cannot reach the SIM")
                textStyle.base: SystemDefaults.TextStyles.TitleText
            }
            Label {
                text: nav.connDetail
                multiline: true
            }
            Button {
                text: qsTr("Try again")
                enabled: !nav.busy
                onClicked: nav.refresh()
            }
        }

        Container {
            visible: nav.conn == "ready"
            layout: StackLayout {}

            Container {
                leftPadding: ui.du(2); rightPadding: ui.du(2); topPadding: ui.du(1); bottomPadding: ui.du(1)
                Label {
                    text: qsTr("EID %1").arg(nav.chip.eid || "")
                    textStyle.base: SystemDefaults.TextStyles.SubtitleText
                    textStyle.color: ui.palette.secondaryTextOnPlain
                }
            }

            Label {
                visible: nav.profiles.length == 0 && !nav.busy
                text: qsTr("No profiles yet. Use Add profile to download one.")
                multiline: true
                horizontalAlignment: HorizontalAlignment.Center
            }

            ListView {
                id: list
                dataModel: ArrayDataModel { id: model }

                function enableProfile(iccid) { page.confirmSwitch(iccid, true) }
                function disableProfile(iccid) { page.confirmSwitch(iccid, false) }
                function busy() { return nav.busy }

                listItemComponents: [
                    ListItemComponent {
                        type: ""
                        StandardListItem {
                            id: item
                            title: ListItemData.nickname || ListItemData.name || ListItemData.provider || ListItemData.iccid
                            description: ListItemData.provider && ListItemData.provider != item.title
                                         ? ListItemData.provider : ListItemData.iccid
                            status: ListItemData.enabled ? qsTr("Enabled") : ""
                            contextActions: [
                                ActionSet {
                                    title: item.title
                                    ActionItem {
                                        title: ListItemData.enabled ? qsTr("Disable") : qsTr("Enable")
                                        imageSource: ListItemData.enabled ? "asset:///images/disable.png" : "asset:///images/enable.png"
                                        enabled: !item.ListItem.view.busy()
                                        onTriggered: {
                                            if (ListItemData.enabled)
                                                item.ListItem.view.disableProfile(ListItemData.iccid);
                                            else
                                                item.ListItem.view.enableProfile(ListItemData.iccid);
                                        }
                                    }
                                }
                            ]
                        }
                    }
                ]

                onTriggered: {
                    var p = dataModel.data(indexPath);
                    if (p)
                        nav.openProfile(p.iccid);
                }
            }
        }
    }

    function confirmSwitch(iccid, enable) {
        switchDialog.iccid = iccid;
        switchDialog.enable = enable;
        switchDialog.body = enable
            ? qsTr("The current profile is disabled and the phone loses service for a moment while it switches.")
            : qsTr("The phone has no mobile service until another profile is enabled.");
        switchDialog.title = enable ? qsTr("Switch to this profile?") : qsTr("Disable this profile?");
        switchDialog.confirmButton.label = enable ? qsTr("Enable") : qsTr("Disable");
        switchDialog.show();
    }

    attachedObjects: [
        SwitchDialog { id: switchDialog }
    ]

    property variant items: nav.profiles
    onItemsChanged: {
        model.clear();
        model.append(items);
    }
    onCreationCompleted: {
        model.clear();
        model.append(items);
    }
}

import bb.cascades 1.4

// Pending SGP.22 notifications: the eUICC queues one for the operator's server
// after each install, enable, disable and delete.
Page {
    id: page

    titleBar: TitleBar { title: qsTr("Notifications") }

    actions: [
        ActionItem {
            title: qsTr("Send all")
            imageSource: "asset:///images/send.png"
            ActionBar.placement: ActionBarPlacement.Signature
            enabled: !nav.busy && nav.notifications.length > 0
            onTriggered: nav.sendNotifications()
        }
    ]

    Container {
        layout: StackLayout {}

        BusyBar {}

        Label {
            leftPadding: ui.du(2); rightPadding: ui.du(2)
            text: qsTr("Operators use these to keep track of their profiles. They are sent automatically after downloads and deletes, and on refresh.")
            multiline: true
            textStyle.base: SystemDefaults.TextStyles.SubtitleText
            textStyle.color: ui.palette.secondaryTextOnPlain
        }

        Label {
            visible: nav.notifications.length == 0
            topMargin: ui.du(3)
            text: qsTr("Nothing pending.")
            horizontalAlignment: HorizontalAlignment.Center
        }

        ListView {
            dataModel: ArrayDataModel { id: model }

            function remove(seq) { nav.removeNotification(seq) }
            function busy() { return nav.busy }

            listItemComponents: [
                ListItemComponent {
                    type: ""
                    StandardListItem {
                        id: item
                        title: ListItemData.operation
                        description: ListItemData.iccid
                        status: "#" + ListItemData.seq
                        contextActions: [
                            ActionSet {
                                DeleteActionItem {
                                    title: qsTr("Discard")
                                    enabled: !item.ListItem.view.busy()
                                    onTriggered: item.ListItem.view.remove(ListItemData.seq)
                                }
                            }
                        ]
                    }
                }
            ]
        }
    }

    // Bound, so the list follows the controller without manual connections.
    property variant items: nav.notifications
    onItemsChanged: {
        model.clear();
        model.append(items);
    }
    onCreationCompleted: {
        model.clear();
        model.append(items);
    }
}

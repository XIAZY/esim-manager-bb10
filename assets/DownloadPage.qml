import bb.cascades 1.4

Page {
    id: page
    objectName: "downloadPage"

    property variant parsed: nav.parseActivationCode(codeField.text)
    property bool manual: manualToggle.checked

    function start() {
        var smdp, matchingId;
        if (manual) {
            smdp = smdpField.text;
            matchingId = matchingIdField.text;
        } else {
            if (!parsed.valid) {
                errorLabel.text = parsed.error;
                return;
            }
            smdp = parsed.smdp;
            matchingId = parsed.matchingId;
        }
        errorLabel.text = "";
        nav.pendingCode = confirmationField.text;
        nav.download(smdp, matchingId);
    }

    function setCode(text) {
        manualToggle.checked = false;
        codeField.text = text;
    }

    titleBar: TitleBar { title: qsTr("Add profile") }

    actions: [
        ActionItem {
            title: qsTr("Download")
            imageSource: "asset:///images/download.png"
            ActionBar.placement: ActionBarPlacement.Signature
            enabled: !nav.busy && (manual || codeField.text != "")
            onTriggered: page.start()
        },
        ActionItem {
            title: qsTr("Scan QR code")
            imageSource: "asset:///images/scan.png"
            ActionBar.placement: ActionBarPlacement.OnBar
            enabled: !nav.busy
            onTriggered: {
                var scan = scanPageDef.createObject();
                scan.scanned.connect(page.setCode);
                nav.push(scan);
            }
        }
    ]

    ScrollView {
        Container {
            layout: StackLayout {}

            BusyBar {}

            Container {
                leftPadding: ui.du(2); rightPadding: ui.du(2); topPadding: ui.du(2)

                Label {
                    text: qsTr("Scan the QR code from your operator, or type the activation code it contains.")
                    multiline: true
                }

                Container {
                    visible: !manual
                    topPadding: ui.du(1)
                    Label { text: qsTr("Activation code") }
                    TextField {
                        id: codeField
                        hintText: "LPA:1$smdp.example.com$MATCHING-ID"
                        inputMode: TextFieldInputMode.Url
                        enabled: !nav.busy
                    }
                }

                Container {
                    visible: manual
                    topPadding: ui.du(1)
                    Label { text: qsTr("SM-DP+ address") }
                    TextField {
                        id: smdpField
                        hintText: qsTr("Empty: the eUICC's default server")
                        inputMode: TextFieldInputMode.Url
                        enabled: !nav.busy
                    }
                    Label { text: qsTr("Matching ID") }
                    TextField {
                        id: matchingIdField
                        hintText: qsTr("Optional")
                        inputMode: TextFieldInputMode.Text
                        enabled: !nav.busy
                    }
                }

                Container {
                    topPadding: ui.du(1)
                    Label {
                        text: !manual && parsed.confirmationCodeRequired
                              ? qsTr("Confirmation code (required)") : qsTr("Confirmation code (if your operator gave you one)")
                        multiline: true
                    }
                    TextField {
                        id: confirmationField
                        inputMode: TextFieldInputMode.Password
                        enabled: !nav.busy
                    }
                }

                Container {
                    topPadding: ui.du(2)
                    layout: StackLayout { orientation: LayoutOrientation.LeftToRight }
                    Label {
                        text: qsTr("Enter server details manually")
                        layoutProperties: StackLayoutProperties { spaceQuota: 1 }
                        verticalAlignment: VerticalAlignment.Center
                    }
                    ToggleButton { id: manualToggle; enabled: !nav.busy }
                }

                Label {
                    id: errorLabel
                    visible: text != ""
                    multiline: true
                    textStyle.color: Color.Red
                }

                Label {
                    topMargin: ui.du(3)
                    text: qsTr("Before anything is installed you see the profile's name and operator and can still cancel.")
                    multiline: true
                    textStyle.base: SystemDefaults.TextStyles.SubtitleText
                    textStyle.color: ui.palette.secondaryTextOnPlain
                }
            }
        }
    }

    attachedObjects: [
        ComponentDefinition { id: scanPageDef; source: "ScanPage.qml" }
    ]
}

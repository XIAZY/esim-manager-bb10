import bb.system 1.2

// Confirms enabling or disabling a profile. Both cut mobile service for a while.
SystemDialog {
    property string iccid
    property bool enable
    cancelButton.label: qsTr("Cancel")
    onFinished: {
        if (value != SystemUiResult.ConfirmButtonSelection)
            return;
        if (enable)
            nav.enableProfile(iccid);
        else
            nav.disableProfile(iccid);
    }
}

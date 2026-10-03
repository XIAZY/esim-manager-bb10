import bb.cascades 1.4

// A labelled read-only value. Hidden when the value is empty.
Container {
    property string label
    property variant value

    visible: value != undefined && value !== ""
    leftPadding: ui.du(2); rightPadding: ui.du(2); topPadding: ui.du(1.5)

    Label {
        text: label
        textStyle.base: SystemDefaults.TextStyles.SubtitleText
        textStyle.color: ui.palette.secondaryTextOnPlain
    }
    Label {
        text: value != undefined ? String(value) : ""
        multiline: true
        topMargin: 0
    }
}

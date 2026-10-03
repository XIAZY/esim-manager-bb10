import bb.cascades 1.4

// Spinner and progress text, shown while the eUICC is busy.
Container {
    visible: nav.busy
    leftPadding: ui.du(2); rightPadding: ui.du(2); topPadding: ui.du(1); bottomPadding: ui.du(1)
    layout: StackLayout { orientation: LayoutOrientation.LeftToRight }

    ActivityIndicator {
        running: nav.busy
        verticalAlignment: VerticalAlignment.Center
    }
    Label {
        text: nav.progressText || qsTr("Working…")
        verticalAlignment: VerticalAlignment.Center
        multiline: true
    }
}

import bb.cascades 1.4
import bb.system 1.2

// Navigation root, and the app's state as the pages see it. Go posts the state
// as JSON through `bridge`; actions go back as commands. Also owns everything
// that must outlive a page: toasts, the download review and the confirmation
// code prompt.
NavigationPane {
    id: nav

    // State from Go (internal/app State).
    property bool busy: false
    property string progressText: ""
    property string conn: "connecting"
    property string connDetail: ""
    property bool previewing: false
    property variant chip: ({})
    property variant profiles: []
    property variant notifications: []

    // Set by DownloadPage before it starts a download.
    property string pendingCode: ""

    function send(name, args) { bridge.send(name, JSON.stringify(args || {})) }
    function refresh() { send("refresh") }
    function enableProfile(iccid) { send("enable", { iccid: iccid }) }
    function disableProfile(iccid) { send("disable", { iccid: iccid }) }
    function deleteProfile(iccid) { send("delete", { iccid: iccid }) }
    function setNickname(iccid, name) { send("nickname", { iccid: iccid, name: name }) }
    function download(smdp, matchingId) { send("download", { smdp: smdp, matchingId: matchingId }) }
    function acceptDownload(code) { send("accept", { code: code }) }
    function rejectDownload() { send("reject") }
    function sendNotifications() { send("sendNotifications") }
    function removeNotification(seq) { send("removeNotification", { seq: String(seq) }) }

    // Splits "LPA:1$smdp$matchingId$oid$ccFlag" (SGP.22 4.1).
    function parseActivationCode(code) {
        var ac = code.replace(/^\s+|\s+$/g, "");
        if (ac.substr(0, 4).toUpperCase() == "LPA:")
            ac = ac.substr(4);
        var f = ac.split("$");
        if (f.length < 2 || f[0] != "1" || f[1].replace(/\s/g, "") == "")
            return { valid: false, error: qsTr("This is not an eSIM activation code. It should look like LPA:1$server$code.") };
        return { valid: true, smdp: f[1], matchingId: f.length > 2 ? f[2] : "",
                 confirmationCodeRequired: f.length > 4 && f[4] == "1" };
    }

    function openProfile(iccid) {
        var page = profilePageDef.createObject();
        page.iccid = iccid;
        nav.push(page);
    }
    function openPage(def) {
        nav.push(def.createObject());
    }
    function popIf(name) {
        if (nav.top && nav.top.objectName == name)
            nav.pop();
    }

    onPopTransitionEnded: page.destroy()

    ProfilesPage {}

    attachedObjects: [
        ComponentDefinition { id: profilePageDef; source: "ProfilePage.qml" },
        ComponentDefinition { id: downloadPageDef; source: "DownloadPage.qml" },
        ComponentDefinition { id: notificationsPageDef; source: "NotificationsPage.qml" },
        ComponentDefinition { id: infoPageDef; source: "InfoPage.qml" },
        SystemToast { id: toast },
        SystemDialog {
            id: previewDialog
            property bool needsCode: false
            title: qsTr("Install this profile?")
            confirmButton.label: qsTr("Install")
            cancelButton.label: qsTr("Cancel")
            onFinished: {
                if (value != SystemUiResult.ConfirmButtonSelection)
                    nav.rejectDownload();
                else if (needsCode && nav.pendingCode == "")
                    codePrompt.show();
                else
                    nav.acceptDownload(nav.pendingCode);
            }
        },
        SystemPrompt {
            id: codePrompt
            title: qsTr("Confirmation code")
            body: qsTr("Your operator requires a confirmation code for this profile.")
            inputField.emptyText: qsTr("Confirmation code")
            confirmButton.label: qsTr("Install")
            cancelButton.label: qsTr("Cancel")
            onFinished: {
                if (value == SystemUiResult.ConfirmButtonSelection && inputFieldTextEntry() != "")
                    nav.acceptDownload(inputFieldTextEntry());
                else
                    nav.rejectDownload();
            }
        }
    ]

    function onState(json) {
        var s = JSON.parse(json);
        busy = s.busy;
        progressText = s.progress;
        conn = s.conn;
        connDetail = s.connDetail;
        previewing = s.previewing;
        chip = s.chip || {};
        profiles = s.profiles || [];
        notifications = s.notifications || [];
    }

    function onEvent(json) {
        var e = JSON.parse(json);
        if (e.type == "toast") {
            toast.body = e.text;
            toast.show();
        } else if (e.type == "preview") {
            var lines = [];
            if (e.name) lines.push(qsTr("Name: %1").arg(e.name));
            if (e.provider) lines.push(qsTr("Provider: %1").arg(e.provider));
            if (e.iccid) lines.push(qsTr("ICCID: %1").arg(e.iccid));
            if (e.profileClass && e.profileClass != "operational") lines.push(qsTr("Class: %1").arg(e.profileClass));
            lines.push(qsTr("Server: %1").arg(e.smdp));
            previewDialog.needsCode = e.codeRequired;
            previewDialog.body = lines.join("\n");
            previewDialog.show();
        } else if (e.type == "installed") {
            popIf("downloadPage");
        } else if (e.type == "deleted") {
            popIf("profilePage");
        }
    }

    onCreationCompleted: {
        bridge.state.connect(onState);
        bridge.event.connect(onEvent);
        refresh();
    }
}

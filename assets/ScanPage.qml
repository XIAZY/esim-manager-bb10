import bb.cascades 1.4
import bb.cascades.multimedia 1.0

// Reads an eSIM activation code (LPA:1$...) from a QR code with the rear
// camera, then hands it back through scanned() and closes.
Page {
    id: page
    objectName: "scanPage"

    signal scanned(string code)
    property bool done: false

    titleBar: TitleBar { title: qsTr("Scan QR code") }

    Container {
        layout: DockLayout {}

        Camera {
            id: camera
            horizontalAlignment: HorizontalAlignment.Fill
            verticalAlignment: VerticalAlignment.Fill

            onCameraOpened: {
                getSettings(settings);
                settings.focusMode = CameraFocusMode.ContinuousAuto;
                applySettings(settings);
                startViewfinder();
            }
            onCameraOpenFailed: status.text = qsTr("The camera could not be opened.")

            attachedObjects: [
                CameraSettings { id: settings },
                BarcodeDetector {
                    camera: camera
                    formats: BarcodeFormat.QrCode
                    onDetected: {
                        if (page.done)
                            return;
                        if (data.substr(0, 4).toUpperCase() != "LPA:") {
                            status.text = qsTr("That QR code is not an eSIM activation code.");
                            return;
                        }
                        page.done = true;
                        camera.stopViewfinder();
                        page.scanned(data);
                        nav.popIf("scanPage");
                    }
                }
            ]
        }

        Container {
            verticalAlignment: VerticalAlignment.Bottom
            horizontalAlignment: HorizontalAlignment.Fill
            background: Color.create("#99000000")
            leftPadding: ui.du(2); rightPadding: ui.du(2); topPadding: ui.du(1.5); bottomPadding: ui.du(1.5)
            Label {
                id: status
                text: qsTr("Point the camera at the QR code from your operator.")
                multiline: true
                textStyle.color: Color.White
            }
        }
    }

    onCreationCompleted: camera.open(CameraUnit.Rear)
}

document.addEventListener('DOMContentLoaded', function () {
    const btnScanExpiry = document.getElementById('btnScanExpiry');
    const expireAtInput = document.getElementById('expireAt') || document.getElementById('inputExpireAt');

    const ocrCameraModal = document.getElementById('ocrCameraModal');
    const ocrResultModal = new bootstrap.Modal(document.getElementById('ocrResultModal'));
    const ocrDetectedDate = document.getElementById('ocrDetectedDate');
    const ocrConfidenceBar = document.querySelector('#ocrResultModal .progress-bar');
    const ocrRawTextWrapper = document.getElementById('ocrRawTextWrapper');
    const ocrRawText = document.getElementById('ocrRawText');
    const btnRecapture = document.getElementById('btnRecaptureExpiry');
    const btnConfirm = document.getElementById('btnConfirmExpiry');
    const btnCapture = document.getElementById('btnCaptureExpiry');

    // Video preview element
    const videoPreview = document.getElementById('ocrVideoPreview');

    let mediaStream = null;

    // Open camera capture modal and start video
    if (btnScanExpiry) {
        btnScanExpiry.addEventListener('click', function () {
            openOcrCamera();
        });
    }

    async function openOcrCamera() {
        // Stop any existing stream
        if (mediaStream) {
            mediaStream.getTracks().forEach(track => track.stop());
            mediaStream = null;
        }

        ocrCameraModal.addEventListener('hidden.bs.modal', onCameraModalHidden, { once: true });
        ocrCameraModal.addEventListener('shown.bs.modal', onCameraModalShown, { once: true });

        const modalInstance = bootstrap.Modal.getOrCreateInstance(ocrCameraModal);
        modalInstance.show();
    }

    function onCameraModalShown() {
        startCamera();
    }

    function onCameraModalHidden() {
        stopCamera();
    }

    async function startCamera() {
        try {
            mediaStream = await navigator.mediaDevices.getUserMedia({
                video: { facingMode: 'environment' },
                audio: false
            });

            if (videoPreview) {
                videoPreview.srcObject = mediaStream;
                videoPreview.onloadedmetadata = function () {
                    videoPreview.play();
                };
            }
        } catch (err) {
            console.error('Camera access error:', err);
            proviant.showFeedback('error', 'Camera Error', 'Could not access camera. Ensure you are on HTTPS or localhost and have granted camera permissions.');
            // Close modal after error
            setTimeout(() => {
                const modal = bootstrap.Modal.getInstance(ocrCameraModal);
                if (modal) modal.hide();
            }, 1500);
        }
    }

    function stopCamera() {
        if (mediaStream) {
            mediaStream.getTracks().forEach(track => track.stop());
            mediaStream = null;
        }
        if (videoPreview) {
            videoPreview.srcObject = null;
        }
    }

    // Capture a frame and run OCR
    if (btnCapture) {
        btnCapture.addEventListener('click', async function () {
            if (!mediaStream || !videoPreview) return;

            try {
                // Create canvas to capture frame
                const canvas = document.createElement('canvas');
                canvas.width = videoPreview.videoWidth;
                canvas.height = videoPreview.videoHeight;
                const ctx = canvas.getContext('2d');
                ctx.drawImage(videoPreview, 0, 0);

                // Convert to blob (JPEG, 0.8 quality)
                const blob = await new Promise(resolve => canvas.toBlob(resolve, 'image/jpeg', 0.8));

                // Stop camera and hide modal
                stopCamera();
                const modalInstance = bootstrap.Modal.getInstance(ocrCameraModal);
                if (modalInstance) modalInstance.hide();

                // Send to OCR
                sendOcrRequest(blob);
            } catch (err) {
                console.error('Capture error:', err);
                proviant.showFeedback('error', 'Capture Failed', 'Could not capture image from camera.');
            }
        });
    }

    async function sendOcrRequest(blob) {
        const formData = new FormData();
        formData.append('image', blob, 'expiry.jpg');

        try {
            const response = await fetch('/api/v1/products/scan-date', {
                method: 'POST',
                body: formData,
            });

            const result = await response.json();

            if (response.ok) {
                showOcrResult(result);
            } else {
                proviant.showFeedback('error', 'OCR Error', result.message || 'Failed to scan expiry date');
            }
        } catch {
            proviant.showFeedback('error', 'Network Error', 'Could not connect to OCR service');
        }
    }

    function showOcrResult(result) {
        // Display detected date
        if (result.detectedDate) {
            ocrDetectedDate.innerHTML = `
                <div class="fw-semibold">Detected: <input type="date" id="ocrDateInput" value="${result.detectedDate}" class="form-control form-control-sm d-inline-block" style="width: auto; min-width: 150px;"></div>
            `;
        } else {
            ocrDetectedDate.innerHTML = `<div class="text-warning">No date detected</div>`;
        }

        // Confidence bar
        const confidencePct = Math.round(result.confidence * 100);
        ocrConfidenceBar.style.width = confidencePct + '%';
        ocrConfidenceBar.classList.toggle('bg-success', result.confidence >= 0.7);
        ocrConfidenceBar.classList.toggle('bg-warning', result.confidence >= 0.4 && result.confidence < 0.7);
        ocrConfidenceBar.classList.toggle('bg-danger', result.confidence < 0.4);
        document.getElementById('ocrConfidenceLabel').textContent = 'Confidence: ' + confidencePct + '%';

        // Raw text for manual correction
        if (result.confidence < 0.7 && result.rawText) {
            ocrRawTextWrapper.classList.remove('d-none');
            ocrRawText.value = result.rawText;
        } else {
            ocrRawTextWrapper.classList.add('d-none');
        }

        ocrResultModal.show();
    }

    // Confirm button — apply date to input field
    if (btnConfirm) {
        btnConfirm.addEventListener('click', function () {
            const dateInput = document.getElementById('ocrDateInput');
            if (dateInput && dateInput.value) {
                expireAtInput.value = dateInput.value;
                expireAtInput.dispatchEvent(new Event('change'));
            }
            ocrResultModal.hide();
        });
    }

    // Retake button — reopen camera
    if (btnRecapture) {
        btnRecapture.addEventListener('click', function () {
            ocrResultModal.hide();
            openOcrCamera();
        });
    }
});

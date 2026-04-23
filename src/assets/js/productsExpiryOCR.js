document.addEventListener('DOMContentLoaded', function () {
    const btnScanExpiry = document.getElementById('btnScanExpiry');
    const expiryImageInput = document.getElementById('expiryImageInput');
    const expireAtInput = document.getElementById('expireAt');
    const ocrResultModal = new bootstrap.Modal(document.getElementById('ocrResultModal'));
    const ocrPreviewImage = document.getElementById('ocrPreviewImage');
    const ocrDetectedDate = document.getElementById('ocrDetectedDate');
    const ocrConfidenceBar = document.querySelector('#ocrResultModal .progress-bar');
    const ocrRawTextWrapper = document.getElementById('ocrRawTextWrapper');
    const ocrRawText = document.getElementById('ocrRawText');
    const btnRecapture = document.getElementById('btnRecaptureExpiry');
    const btnConfirm = document.getElementById('btnConfirmExpiry');

    let capturedImageBlob = null;

    // Open camera capture
    if (btnScanExpiry) {
        btnScanExpiry.addEventListener('click', function () {
            expiryImageInput.click();
        });
    }

    expiryImageInput.addEventListener('change', function (event) {
        const file = event.target.files[0];
        if (!file) return;

        // Read preview
        const reader = new FileReader();
        reader.onload = function (e) {
            ocrPreviewImage.innerHTML = `<img src="${e.target.result}" class="img-fluid rounded" style="max-height: 200px;">`;
            capturedImageBlob = file;
            sendOcrRequest(file);
        };
        reader.readAsDataURL(file);
    });

    async function sendOcrRequest(file) {
        const formData = new FormData();
        formData.append('image', file);

        try {
            const response = await fetch('/api/v1/products/scan-date', {
                method: 'POST',
                body: formData,
                headers: { 'Accept': 'application/json' } // fetch will set multipart/form-data with boundary automatically
            });

            const result = await response.json();

            if (response.ok) {
                showOcrResult(result);
            } else {
                proviant.showFeedback('error', 'OCR Error', result.message || 'Failed to scan expiry date');
            }
        } catch (error) {
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
                // Trigger validation update
                expireAtInput.dispatchEvent(new Event('change'));
            }
            ocrResultModal.hide();
        });
    }

    // Retake button — reopen camera
    if (btnRecapture) {
        btnRecapture.addEventListener('click', function () {
            ocrResultModal.hide();
            expiryImageInput.value = ''; // reset
            expiryImageInput.click();
        });
    }

    // Also allow date input edit in modal to propagate on blur
    document.getElementById('ocrDateInput')?.addEventListener('change', function (e) {
        // just updates local state, will be applied on confirm
    });
});

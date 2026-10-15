// app.js - Interactive UI and Modal Handlers

document.addEventListener('DOMContentLoaded', function () {
  // 1. Slip Image Preview Modal
  const previewModal = document.getElementById('slipPreviewModal');
  if (previewModal) {
    previewModal.addEventListener('show.bs.modal', function (event) {
      const button = event.relatedTarget;
      const slipUrl = button.getAttribute('data-slip-url');
      const imgTarget = document.getElementById('slipPreviewImage');
      const pdfTarget = document.getElementById('slipPreviewPdf');
      const downloadBtn = document.getElementById('slipDownloadBtn');

      if (slipUrl.endsWith('.pdf')) {
        imgTarget.classList.add('d-none');
        pdfTarget.classList.remove('d-none');
        pdfTarget.setAttribute('src', slipUrl);
      } else {
        pdfTarget.classList.add('d-none');
        imgTarget.classList.remove('d-none');
        imgTarget.setAttribute('src', slipUrl);
      }

      if (downloadBtn) {
        downloadBtn.setAttribute('href', slipUrl);
      }
    });
  }

  // 2. Edit Room Modal Populate
  const editRoomModal = document.getElementById('editRoomModal');
  if (editRoomModal) {
    editRoomModal.addEventListener('show.bs.modal', function (event) {
      const button = event.relatedTarget;
      const id = button.getAttribute('data-id');
      const number = button.getAttribute('data-number');
      const type = button.getAttribute('data-type');
      const price = button.getAttribute('data-price');
      const floor = button.getAttribute('data-floor');
      const status = button.getAttribute('data-status');
      const desc = button.getAttribute('data-desc');

      const form = document.getElementById('editRoomForm');
      form.action = `/rooms/${id}/edit`;
      document.getElementById('edit_room_number').value = number;
      document.getElementById('edit_room_type').value = type;
      document.getElementById('edit_price').value = price;
      document.getElementById('edit_floor').value = floor;
      document.getElementById('edit_status').value = status;
      document.getElementById('edit_description').value = desc || '';
    });
  }

  // 3. Maintenance Status Modal Populate
  const editMaintModal = document.getElementById('editMaintModal');
  if (editMaintModal) {
    editMaintModal.addEventListener('show.bs.modal', function (event) {
      const button = event.relatedTarget;
      const id = button.getAttribute('data-id');
      const status = button.getAttribute('data-status');
      const notes = button.getAttribute('data-notes');
      const title = button.getAttribute('data-title');

      const form = document.getElementById('editMaintForm');
      form.action = `/maintenance/${id}/status`;
      document.getElementById('maint_modal_title').textContent = title;
      document.getElementById('maint_status').value = status;
      document.getElementById('maint_notes').value = notes || '';
    });
  }

  // 4. Payment Review Modal Populate
  const reviewPaymentModal = document.getElementById('reviewPaymentModal');
  if (reviewPaymentModal) {
    reviewPaymentModal.addEventListener('show.bs.modal', function (event) {
      const button = event.relatedTarget;
      const id = button.getAttribute('data-id');
      const tenant = button.getAttribute('data-tenant');
      const month = button.getAttribute('data-month');
      const amount = button.getAttribute('data-amount');
      const status = button.getAttribute('data-status');
      const reason = button.getAttribute('data-reason');
      const slipUrl = button.getAttribute('data-slip');

      const form = document.getElementById('reviewPaymentForm');
      form.action = `/payments/${id}/status`;
      document.getElementById('payment_tenant_name').textContent = tenant;
      document.getElementById('payment_info').textContent = `${month} - ${Number(amount).toLocaleString()} บาท`;
      document.getElementById('payment_status').value = status;
      document.getElementById('payment_reason').value = reason || '';

      const slipLink = document.getElementById('payment_slip_link');
      if (slipUrl && slipUrl !== '') {
        slipLink.href = slipUrl;
        slipLink.classList.remove('d-none');
      } else {
        slipLink.classList.add('d-none');
      }
    });
  }

  // 5. Auto dismiss alerts after 5 seconds
  setTimeout(function () {
    const alerts = document.querySelectorAll('.alert-dismissible');
    alerts.forEach((alert) => {
      const bsAlert = new bootstrap.Alert(alert);
      bsAlert.close();
    });
  }, 5000);
});

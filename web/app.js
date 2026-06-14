async function sendScan() {
    const rootPath = document.getElementById('rootPath').value;
    const resultElement = document.getElementById('result');

    try {
        const response = await fetch('/api/scan', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ root_path: rootPath }),
        });

        const data = await response.json();
        resultElement.innerText = data.status || 'İşlem başlatıldı';
    } catch (error) {
        console.error('Hata:', error);
        resultElement.innerText = 'Sunucuyla iletişim kurulurken bir hata oluştu.';
    }
}

async function deleteFile(id) {
    if (!confirm('Bu dosyayı fiziksel olarak ve veritabanından silmek istediğinize emin misiniz?')) return;
    try {
        const response = await fetch(`/api/delete?id=${id}`, { method: 'DELETE' });
        if (response.ok) {
            location.reload();
        } else {
            const errorText = await response.text();
            alert('Silme işlemi başarısız: ' + errorText);
        }
    } catch (e) { 
        console.error(e); 
        alert('Sunucuyla iletişim kurulurken bir hata oluştu: ' + e.message);
    }
}
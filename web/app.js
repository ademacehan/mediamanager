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

async function renameFile(id, currentName, type, unixMilli) {
    const date = new Date(unixMilli);
    const pad = (n, l = 2) => String(n).padStart(l, '0');
    
    const y = date.getFullYear();
    const m = pad(date.getMonth() + 1);
    const d = pad(date.getDate());
    const hh = pad(date.getHours());
    const mm = pad(date.getMinutes());
    const ss = pad(date.getSeconds());
    const ms = pad(date.getMilliseconds(), 3);

    const prefix = type === 'image' ? 'IMG' : 'VID';
    const timePart = `${y}${m}${d}_${hh}${mm}${ss}_${ms}`;
    
    const lastDotIndex = currentName.lastIndexOf('.');
    const base = lastDotIndex !== -1 ? currentName.substring(0, lastDotIndex) : currentName;
    const ext = lastDotIndex !== -1 ? currentName.substring(lastDotIndex).toLowerCase() : "";

    const suggestedName = `${prefix}_${timePart}_${base}${ext}`;
    const newName = prompt("Yeni dosya adını girin (uzantısıyla birlikte):", suggestedName);
    if (!newName || newName === currentName) return;
    const res = await fetch(`/api/rename?id=${id}&new_name=${encodeURIComponent(newName)}`, { method: 'POST' });
    if (res.ok) {
        location.reload();
    } else {
        alert("Hata: " + await res.text());
    }
}

async function moveFile(id, hashID, type, year, month, day) {
    // Bu değişkenlerin HTML içerisinde (index.html vb.) <script> bloğunda tanımlanmış olması gerekir.
    const imgBase = window.imageSuggestBase || "";
    const vidBase = window.videoSuggestBase || "";
    
    const base = type === 'image' ? imgBase : vidBase;
    // Path separator'ın (\\) düzgün oluştuğundan emin olalım
    const suggestedPath = base + "\\" + year + "\\" + month + "\\" + day;
    const targetDir = prompt("Dosyanın taşınacağı hedef dizin yolunu girin:", suggestedPath);
    if (!targetDir) return;
    const res = await fetch(`/api/move?id=${id}&target_dir=${encodeURIComponent(targetDir)}`, { method: 'POST' });
    if (res.ok) {
        await autoTagFromDate(hashID, year, month, day, true);
        location.reload();
    } else {
        alert("Hata: " + await res.text());
    }
}

async function addTag(hashID) {
    const defTag = window.defaultHashtag || "";
    const lastUsedTag = localStorage.getItem('lastUsedHashtag') || defTag;
    const tag = prompt("Eklemek istediğiniz etiketleri girin (virgül ile ayırabilirsiniz):", lastUsedTag);
    if (!tag) return;
    localStorage.setItem('lastUsedHashtag', tag);
    const res = await fetch(`/api/add-tag?hash_id=${hashID}&tag=${encodeURIComponent(tag)}`, { method: 'POST' });
    if (res.ok) {
        location.reload();
    } else {
        alert("Hata: " + await res.text());
    }
}

async function removeTag(hashID, tag) {
    if (!confirm(`'${tag}' etiketini bu resimden kaldırmak istediğinize emin misiniz?`)) return;
    const res = await fetch(`/api/remove-tag?hash_id=${hashID}&tag=${encodeURIComponent(tag)}`, { method: 'POST' });
    if (res.ok) {
        location.reload();
    } else {
        alert("Hata: " + await res.text());
    }
}

async function autoTagFromDate(hashID, year, month, day, skipReload = false) {
    const months = {
        "01": "Ocak", "02": "Şubat", "03": "Mart", "04": "Nisan",
        "05": "Mayıs", "06": "Haziran", "07": "Temmuz", "08": "Ağustos",
        "09": "Eylül", "10": "Ekim", "11": "Kasım", "12": "Aralık"
    };
    
    // Ay bilgisinin 2 haneli (01, 02 vb.) olduğundan emin olalım
    const monthKey = String(month).padStart(2, '0');
    const tags = `${year},${months[monthKey] || month},${day}`;
    const res = await fetch(`/api/add-tag?hash_id=${hashID}&tag=${encodeURIComponent(tags)}`, { method: 'POST' });
    if (res.ok) {
        if (!skipReload) location.reload();
    } else {
        alert("Hata: " + await res.text());
    }
}

async function deleteOthers(keepID, hashID) {
    if (!confirm('Bu dosya DIŞINDAKI tüm kopyaları hem diskten hem veritabanından silmek istediğinize emin misiniz?')) return;
    try {
        const res = await fetch(`/api/delete-others?keep_id=${keepID}&hash_id=${hashID}`, { method: 'DELETE' });
        if (res.ok) {
            location.reload();
        } else {
            alert("Hata: " + await res.text());
        }
    } catch (e) {
        alert("Sunucuyla iletişim kurulurken bir hata oluştu.");
    }
}

async function moveAndDeleteOthers(id, hashID, type, year, month, day) {
    const imgBase = window.imageSuggestBase || "";
    const vidBase = window.videoSuggestBase || "";
    
    const base = type === 'image' ? imgBase : vidBase;
    const suggestedPath = base + "\\" + year + "\\" + month + "\\" + day;
    const targetDir = prompt("DİKKAT: Bu işlem dosyayı taşıyacak ve DİĞER tüm kopyaları silecektir.\nHedef dizin yolunu girin:", suggestedPath);
    if (!targetDir) return;
    try {
        const res = await fetch(`/api/move-and-delete-others?id=${id}&hash_id=${hashID}&target_dir=${encodeURIComponent(targetDir)}`, { method: 'POST' });
        if (res.ok) {
            await autoTagFromDate(hashID, year, month, day, true);
            location.reload();
        } else {
            alert("Hata: " + await res.text());
        }
    } catch (e) { alert("Sunucuyla iletişim kurulurken bir hata oluştu."); }
}
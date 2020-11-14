let map = L.map("map", {
    zoomSnap: 0,
    zoomAnimation: false,
    center: L.latLng(gastrakLat, gastrakLong),
    zoom: 12,
});

L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 16,
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
}).addTo(map);

function getNavigationUrl(lat, long) {
    if (/Android/.test(navigator.userAgent)) {
        return `google.navigation:q=${lat},${long}`;
    } else if (/iPhone|iPad|iPod/.test(navigator.userAgent)) {
        return `http://maps.apple.com/?daddr=${lat},${long}&dirflg=d`;
    } else {
        return `https://www.google.com/maps/?q=@${lat},${long}`;
    }
}

let lines = gastrakData.split('\n').filter((l) => l !== '');
for (let line of lines) {
    let cols = line.split(',');
    let name = cols[2];
    let lat = parseFloat(cols[3]);
    let long = parseFloat(cols[4]);
    let price = cols[5];
    let url = getNavigationUrl(lat, long);
    L.marker([lat, long]).bindTooltip(
        `<a href="${url}"><b>${name}</b></a><br>\$${price}`, {
            direction: "top",
            permanent: true,
            offset: L.point(0, -16),
            opacity: 1,
            interactive: true,
        }
    ).addTo(map);
}

let mapDiv = document.getElementById("map");
let historyDialog = document.getElementById("history");

function createElement(tag, attrs, children) {
    let elem = document.createElement(tag);
    if (attrs) {
        for (let prop in attrs) {
            elem[prop] = attrs[prop];
        }
    }
    if (children) {
        for (let child of children) {
            elem.append(child);
        }
    }
    return elem;
}

function getNavigationUrl(lat, lng) {
    if (/Android/.test(navigator.userAgent)) {
        return `google.navigation:q=${lat},${lng}`;
    } else if (/iPhone|iPad|iPod/.test(navigator.userAgent)) {
        return `https://maps.apple.com/?daddr=${lat},${lng}&dirflg=d`;
    } else {
        return `https://www.google.com/maps/?q=@${lat},${lng}`;
    }
}

function showHistory(name, grade, data) {
    historyDialog.innerHTML = "";

    Highcharts.chart(historyDialog, {
        chart: {type: "line", zoomType: "x"},
        title: {text: `${name} (${grade})`},
        xAxis: {type: "datetime"},
        yAxis: {
            labels: {format: "${value:.2f}"},
            title: {text: "Price"},
            min: 0
        },
        legend: {enabled: false},
        series: [{name: name, data: data}],
    });

    historyDialog.show();
}

async function fetchAndShowHistory(name, grade) {
    let resp = await fetch("/history?" + new URLSearchParams({
        name: name,
        grade: grade,
        format: "highcharts",
    }).toString());
    let data = JSON.parse(await resp.text());
    showHistory(name, grade, data);
}

function showMap(lat, lng, datas) {
    let map = L.map(mapDiv, {
        zoomSnap: 0,
        zoomAnimation: false,
        center: L.latLng(lat, lng),
        zoom: 11,
    });

    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        maxZoom: 16,
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    }).addTo(map);

    for (let data of datas) {
        let station = data["Station"]
        let name = station["Name"];
        let lat = station["Latitude"];
        let lng = station["Longitude"];
        let price = data["RegularPrice"];
        let url = getNavigationUrl(lat, lng);
        let onclick = async () => { await fetchAndShowHistory(name, "regular"); };

        let tooltip = createElement("div", {}, [
            createElement("a", {href: url}, [
                createElement("b", {}, [name])
            ]),
            createElement("br"),
            createElement("a", {onclick: onclick}, [
                "$" + price + " "
            ]),
        ]);

        L.marker([lat, lng]).bindTooltip(tooltip, {
            direction: "top",
            permanent: true,
            offset: L.point(-16, -16),
            opacity: 1,
            interactive: true,
        }).addTo(map);
    }
}

document.body.addEventListener("mousedown", (e) => {
    if (e.target.closest("dialog") === null) {
        historyDialog.innerHTML = "";
        historyDialog.close();
    }
});

showMap(gastrak["Latitude"], gastrak["Longitude"], gastrak["Data"]);

let map = L.map("map", {
    zoomSnap: 0,
    zoomAnimation: false,
    center: L.latLng(gastrak["Latitude"], gastrak["Longitude"]),
    zoom: 11,
});

L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 16,
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
}).addTo(map);

function getNavigationUrl(lat, long) {
    if (/Android/.test(navigator.userAgent)) {
        return `google.navigation:q=${lat},${long}`;
    } else if (/iPhone|iPad|iPod/.test(navigator.userAgent)) {
        return `https://maps.apple.com/?daddr=${lat},${long}&dirflg=d`;
    } else {
        return `https://www.google.com/maps/?q=@${lat},${long}`;
    }
}

function createElement(tag, attrs, children) {
    let elem = document.createElement(tag);
    if (attrs) {
        for (let prop in attrs) {
            elem[prop] = attrs[prop];
        }
    }
    if (children) {
        for (let child of children) {
            elem.appendChild(child);
        }
    }
    return elem;
}

async function showHistory(name) {
    let dialog = document.getElementById("history");
    dialog.innerHTML = "";

    let resp = await fetch("/history?" + new URLSearchParams({
        "name": name,
        "format": "json"
    }).toString());
    let json = JSON.parse(await resp.text());

    let points = [];
    for (let data of json) {
        points.push([
            Date.parse(data["Timestamp"]),
            data["RegularPrice"]
        ]);
    }

    Highcharts.chart(dialog, {
        "chart": {"type": "line", "zoomType": "x"},
        "title": {"text": name},
        "xAxis": {"type": "datetime"},
        "yAxis": {
            "labels": {"format": "${value:.2f}"},
            "title": {"text": "Price"},
            "min": 0
        },
        "legend": {"enabled": false},
        "series": [{
            "name": name,
            "data": points,
        }],
    });

    dialog.show();
}

for (let data of gastrak["Data"]) {
    let name = data["Name"];
    let lat = data["Latitude"];
    let long = data["Longitude"];
    let price = data["RegularPrice"];
    let url = getNavigationUrl(lat, long);

    let tooltip = createElement("div", {}, [
        createElement("a", {"href": url}, [
            createElement("b", {"innerText": name})
        ]),
        createElement("br"),
        createElement("span", {"innerText": `\$${price.toString()} `}),
        createElement("a", {
            "onclick": async () => { await showHistory(name); },
            "innerText": "^",
        }),
    ]);

    L.marker([lat, long]).bindTooltip(tooltip, {
        direction: "top",
        permanent: true,
        offset: L.point(0, -16),
        opacity: 1,
        interactive: true,
    }).addTo(map);
}

document.body.addEventListener("mousedown", (e) => {
    if (e.target.closest("dialog") === null) {
        let dialog = document.getElementById("history");
        dialog.close();
    }
});

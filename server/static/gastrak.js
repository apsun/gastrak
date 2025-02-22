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

    let plot = new uPlot({
        title: `${name} (${grade})`,
        width: document.documentElement.clientWidth * 0.8,
        height: document.documentElement.clientHeight * 0.6,
        series: [
            {},
            {
                label: "Price",
                stroke: "blue",
                value: (u, v) => `\$${v}`,
            },
        ],
        axes: [
            {},
            {
                values: (u, vs) => vs.map(v => `\$${v}`),
            },
        ],
    }, data, historyDialog);
    historyDialog.plot = plot;

    historyDialog.resizeListener = () => {
        plot.setSize({
            width: document.documentElement.clientWidth * 0.8,
            height: document.documentElement.clientHeight * 0.6,
        });
    };
    window.addEventListener("resize", historyDialog.resizeListener);

    historyDialog.show();
}

async function fetchAndShowHistory(name, grade) {
    let resp = await fetch("/history?" + new URLSearchParams({
        name: name,
        grade: grade,
        format: "timeseries-transposed",
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
        let name = data["Name"];
        let lat = data["Latitude"];
        let lng = data["Longitude"];
        let price = data["RegularPrice"];
        if (price === undefined) {
            price = "N/A";
        } else {
            price = "$" + price;
        }

        let url = getNavigationUrl(lat, lng);
        let onclick = async () => { await fetchAndShowHistory(name, "regular"); };

        let tooltip = createElement("div", {}, [
            createElement("a", {href: url}, [
                createElement("b", {}, [name]),
            ]),
            createElement("br"),
            createElement("a", {onclick: onclick}, [price]),
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
        if (historyDialog.resizeListener) {
            window.removeEventListener("resize", historyDialog.resizeListener);
            delete historyDialog.resizeListener;
        }
        if (historyDialog.plot) {
            historyDialog.plot.destroy();
            delete historyDialog.plot;
        }
        historyDialog.innerHTML = "";
        historyDialog.close();
    }
});

document.title = "gastrak @ " + new Date(gastrak["Time"]).toLocaleDateString();

showMap(gastrak["Latitude"], gastrak["Longitude"], gastrak["Data"]);

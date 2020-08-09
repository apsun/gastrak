#![feature(proc_macro_hygiene, decl_macro)]

extern crate chrono;
#[macro_use]
extern crate rocket;
extern crate rocket_contrib;
extern crate structopt;

use chrono::DateTime;
use chrono::Utc;
use rocket::State;
use rocket_contrib::serve::StaticFiles;
use rocket_contrib::templates::Template;
use std::collections::HashMap;
use std::convert::Into;
use std::fs;
use structopt::StructOpt;

#[derive(Debug, StructOpt)]
struct Config {
    #[structopt(long)]
    latitude: f64,

    #[structopt(long)]
    longitude: f64,

    #[structopt(long)]
    data: String,
}

#[get("/")]
fn index(cfg: State<Config>) -> Template {
    let ts: DateTime<Utc> = fs::metadata(&cfg.data).unwrap().modified().unwrap().into();
    let mut context = HashMap::<&'static str, String>::new();
    context.insert("latitude", cfg.latitude.to_string());
    context.insert("longitude", cfg.longitude.to_string());
    context.insert("data", fs::read_to_string(&cfg.data).unwrap());
    context.insert("time", ts.format("%F").to_string());
    Template::render("index", &context)
}

fn main() {
    let cfg = Config::from_args();
    rocket::ignite()
        .manage(cfg)
        .mount("/", routes![index])
        .mount("/static", StaticFiles::from("static"))
        .attach(Template::fairing())
        .launch();
}

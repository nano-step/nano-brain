import { readFile } from "fs";
const path = require("path");

class Router {
  handle(req) {
    return this.dispatch(req);
  }
}

function processRequest(req) {
  const data = parseBody(req);
  console.log(data);
  return data;
}

function parseBody(req) {
  return JSON.parse(req.body);
}

const config = { port: 3000 };

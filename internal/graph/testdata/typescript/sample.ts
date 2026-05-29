import { readFile } from "fs";
import path from "path";
const lodash = require("lodash");

interface Config {
  port: number;
}

type Handler = (req: Request) => Response;

enum Status {
  Active,
  Inactive,
}

class Server {
  start() {
    console.log("starting");
  }
}

function handleRequest(req: Request): Response {
  const result = processData(req);
  return result;
}

function processData(data: any): any {
  return data;
}

const helper = "not a function";

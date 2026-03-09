import fs from "fs/promisify";
import path from "path";
import jsonfile from "jsonfile";
import moment from "moment";
import simpleGit from "simple-git";
import { promisify } from "util";
import { faker } from "@faker-js/faker";

const git = simpleGit();

await fs.mkdir("./dataStorage", { recursive: true });

const dailySchedule = [];
for (let day = 1; day <= 30; day++) {
  dailySchedule.push({ day, iterations: Math.floor(Math.random() * 10) + 1 });
}

const dataFiles = [
  "./dataStorage/data1.json",
  "./dataStorage/data2.json",
  "./dataStorage/data3.json",
  "./dataStorage/data4.json",
  "./dataStorage/data5.json",
];

const writeFileAsync = promisify(jsonfile.writeFile);

const makeCommits = async (repeats, iterationsPerRepeat) => {
  console.log(
    `Начало: ${repeats} повторений, ${iterationsPerRepeat} итераций на повторение`,
  );

  for (let r = 0; r < repeats; r++) {
    console.log(`Повторение ${r + 1}/${repeats}`);

    for (let iter = 0; iter < iterationsPerRepeat; iter++) {
      console.log(`Итерация ${iter + 1}/${iterationsPerRepeat}`);

      const dayData =
        dailySchedule[Math.floor(Math.random() * dailySchedule.length)];

      const fakeData = generateFakeData();

      await createDailyCommit(fakeData, dayData.day, dayData.iterations);
    }
  }

  console.log("Завершено");
};

function generateFakeData() {
  const data = [];

  for (let i = 0; i < 5; i++) {
    data.push({
      x: faker.number.int({ min: 1, max: 55 }),
      y: faker.number.int({ min: 1, max: 7 }),
      value: faker.number.float({ min: 10, max: 100, precision: 0.01 }),
      name: faker.commerce.productName(),
      timestamp: faker.date.recent().toISOString(),
      time: Array.from({ length: 5 }, () => ({
        hour: faker.number.int({ min: 0, max: 23 }),
        minute: faker.number.int({ min: 0, max: 59 }),
        second: faker.number.int({ min: 0, max: 59 }),
      })),
    });
  }

  return data;
}

async function createDailyCommit(fakeDataCommits, dayNum, totalIterations) {
  const allPaths = dataFiles;

  try {
    const writePromise = allPaths.map(async (filePath, i) => {
      const timeCommit = fakeDataCommits[i].time;
      const randomTimeIdx = Math.floor(Math.random() * timeCommit.length);

      const date = moment()
        .subtract(1, "year")
        .add(dayNum, "days")
        .add(fakeDataCommits[i].x || 0, "weeks")
        .add(fakeDataCommits[i].y || 0, "days")
        .set(timeCommit[randomTimeIdx])
        .format();

      const dbData = {
        ...fakeDataCommits[i],
        date: date,
        iteration: totalIterations,
        day: dayNum,
      };

      await writeFileAsync(filePath, dbData);
      console.log(`Записано в ${path.basename(filePath)}: ${data}`);

      return { path: filePath, date };
    });

    const results = await Promise.all(writePromise);

    await git.add(allPaths);
    const commitDate = results[0].date;
    await git.commit(commitDate, { "--date": commitDate });
    await git.push();

    console.log(`Коммит всех файлов: ${commitDate}`);

    await new Promise((resolve) => setTimeout(resolve, 500));
  } catch (error) {
    console.log("Ошибка в коммите", error);
  }
}

makeCommits(3, 5);

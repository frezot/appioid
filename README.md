# Appiod
Tool for manage android devices and appium servers

## Requirements
* [npm](https://www.npmjs.com/get-npm)
* appium CLI `npm install -g appium`
* android platform-tools [ReadMore](https://stackoverflow.com/questions/20564514/adb-is-not-recognized-as-an-internal-or-external-command-operable-program-or)

## Quick start
* just download binary file and launch it
* (Windows) if you want to run appioid as a service — use [NSSM](https://github.com/kirillkovalenko/nssm) 

## Appioid arguments
| Flag   | Default | Description                                                         |
| ------ |:-------:| ------------------------------------------------------------------- |
| `-p`   | 9093    | Port to listen on                                                   |
| `-sz`  | 2       | How much appium servers should works at same time                   |
| `-TTL` | 300     | Max time (in seconds) which node or device might be in use          |
| `-rd`  |         | Reserved device (This deviceName never be returned by `/getDevice`) |
| `-ap`  | 4725    | First value of appiumPort counter                                   |
| `-sp`  | 8202    | First value of systepPort counter                                   |


***

## Идея коротко:
* есть некоторое число андроид-девайсов (или эмуляторов) подключенных к хосту
* хотим тестироваться средствами appium-а параллельно

## Понадобится:
* иметь возможность занять девайс, освободить, запросить свободный
* для каждого девайса поднять свой appium на своем порту
* заложить таймауты, чтобы развалившийся тест не испортил жизнь остальным
* помнить какие порты выдаются под аппиум чтобы овербукинг не получился
* контролировать что аппиум был адекватно завершен, а не повис навечно
* повисшие процессы отлавливать и прибивать
* быть уверенным в стабильности самой системы контроля
* общаться с сиcтемой через http, максимально просто


Приблизительно поэтому было решено написать данную тулзу

import type { answerMessage, checkUserActive, iceCandidate, startStreamMessage, StatusMessage } from "~~/types/signaling";
import { Signaling } from "./Signaling";
import { User } from "./User";

export class WebRTC extends Signaling {
    #User
    #callStore = callStore()
    #stream
    #idRoom
    #webSoket: WebSocket
    constructor(idRoom: string, typeWS: string) {
        let webSoket = new WebSocket(`ws://localhost:8090/ws/${idRoom}/${typeWS}`)
        webSoket.onerror = async () => {
            const delay = (ms: any) => new Promise(resolve => setTimeout(resolve, ms));
            const items = [1, 2, 3, 4, 5]
            for (const el of items) {

                console.log("Попытка подключиться")
                webSoket = await new WebSocket(`ws://localhost:8090/ws/${idRoom}/${typeWS}`)
                await delay(3000)
            }
            console.log("Не удалось подключиться")
            return false
        }
        const User = useUserStore()
        super(webSoket, String(User.userData != undefined ? User.userData.id : -1), idRoom)
        this.#User = User
        this.#webSoket = webSoket
        this.#idRoom = idRoom
    }

    #initializationWS() {
        this.#webSoket.onmessage = async (event) => {
            const data = JSON.parse(event.data)
            console.log(data)
            console.log(typeof data)
            if (data.type == "StartSream") {
                (data as startStreamMessage)
                this.#callStore.settingCall = data.system_option
                this.sendSignalStatusUser("Expectation")
            } else if (data.type == "Status" && data.name !== this.#User.userData.userName && data.idRoom == this.#idRoom) {
                (data as StatusMessage)
                if (data.statusUser == "Expectation") {
                    this.#reactionNewUser(data.idUserTarget)
                } else if (data.statusUser == "Active") {
                    this.#reactionActiveUser(data, data.idUserTarget, data.offer ? data.offer : "")
                } else if (data.statusUser == "Close") {
                    this.#closeCall(data)
                }


            } else if (data.type == "Answer" && data.idUserAnswer == this.#User.userData.id || data.idUserAnswer == -1) {
                (data as answerMessage)
                if (data.status === "Sent") {
                    if (data.action === "Active") {
                        this.#reactionAnswerActive(data)
                    } else if (data.action === "Expectation") {
              
                    } else if (data.action === "checkUserActive") {

                        await this.#reactionAnswerCheckUser(data)
                    } else if (data.action === "Close") {
                        this.#reactionAnswerClose(data)
                    }
                }
            } else if (data.type == "iceCandidate") {
                (data as iceCandidate)
                this.#reactiionICECandidate(data)
            } else if (data.type == "checkUserActive") {
                (data as checkUserActive)
                console.log(data.preliminary)
                if (!data.preliminary) {
                    console.log("Чек от пользователя: ")
                    console.log(data)
                    console.log(data.idRoom === this.#idRoom)
                    console.log(this.#callStore.peerConnectionUsers.has(data.idUserTarget))
                    if (!this.#callStore.peerConnectionUsers.has(data.idUserTarget) && data.idRoom === this.#idRoom) {
                        this.#reactionCheckUser(data)
                    } else {
                        return 0
                    }
                } else {
                    console.log(data)
                    this.#webSoket.send(JSON.stringify({
                        type: "Answer",
                        action: "checkUserActive",
                        preliminary: true,
                        idUserTarget: this.#User.userData?.id,
                        idUserAnswer: data.idUserTarget,
                        idRoom: this.#idRoom
                    }))
                }
            }
        }
    }

    #closeCall(data: object) {

        const pcInfo = this.#callStore.peerConnectionUsers.get(data.idUserTarget);
        if (!pcInfo) return;

        pcInfo.PeerConectionICE.getSenders().forEach(sender => {
            pcInfo.PeerConectionICE.removeTrack(sender);
        });

        pcInfo.PeerConectionICE.close();
        this.#callStore.peerConnectionUsers.delete(data.idUserTarget);

        this.sendSignalAnswer(data.idUserTarget, this.#User.userData.id, "Close", "Sent")
    }

    #reactionNewUser(id: string) {
        const user = (new User()).getUserById(id)

        this.#callStore.expectationUserCall.push(user)
    }
    async #reactionActiveUser(data: object, id: string, offer: object | "" = "") {
        this.#callStore.expectationUserCall.forEach((user, id) => {
            if (user.id == id) {
                this.#callStore.activeUserCall.push(user)
                this.#callStore.expectationUserCall.slice(id, 1)
            }
        })

        if (offer && data.idUserAnswer == this.#User.userData.id && this.#callStore.newUserPC(data).PeerConectionICE.signalingState === "stable") {
            console.log("AnswerActiveData: ", data)
            await this.#callStore.newUserPC(data).PeerConectionICE.setRemoteDescription(new RTCSessionDescription(data.offer));
            const answer = await this.#callStore.newUserPC(data).PeerConectionICE.createAnswer();
            await this.#callStore.newUserPC(data).PeerConectionICE.setLocalDescription(answer);
            this.sendSignalAnswer(data.idUserTarget, data.idUserAnswer, "Active", "Sent", answer)
        } else {
            this.sendSignalAnswer(data.idUserTarget, data.idUserAnswer, "Active", "Sent")
        }
    }


    #reactionAnswerActive(data: object) {
        console.log("reactionAnswerActive: ", data)
        if (data.answer) {
            this.#callStore.newUserPC(data).PeerConectionICE.setRemoteDescription(new RTCSessionDescription(data.answer))
        }
    }

    async #reactionAnswerCheckUser(data: object) {
        if (this.#callStore.newUserPC(data, this.#User.userData, this.#stream) === undefined) {
            console.log("Получен ответ на чек: ", data)
            const pc = this.#callStore.newUserPC(data).PeerConectionICE
            console.log("Объект пользователя, который ответил: ", pc)
            await this.#callStore.newUserPC(data).PeerConectionICE.setRemoteDescription(new RTCSessionDescription(data.answer))
            console.log(this.#callStore.newUserPC(data))
            const answer = await this.#callStore.newUserPC(data).PeerConectionICE.createAnswer()
            await this.#callStore.newUserPC(data).PeerConectionICE.setLocalDescription(answer)
            console.log("Отправили ответ: ", data.idUserTarget)
            setTimeout(async () => {
                console.log("ЖДЁМС")
                console.log(this.#callStore.newUserPC(data).ICEcandidate)
                const iceCandidate = await this.#callStore.newUserPC(data).ICEcandidate
                console.log(iceCandidate.length)
                iceCandidate.forEach((ice) => {
                    console.log("ОТправленный кандидат: ", ice)
                    setTimeout(() => {
                        this.sendSignalICECandidate(ice)
                    }, 80)
                })
            }, 500)
            this.sendSignalAnswer(data.idUserTarget, data.idUserAnswer, "checkUserActive", "Sent", answer)
        } else if (this.#callStore.newUserPC(data) !== undefined && this.#callStore.newUserPC(data).PeerConectionICE.signalingState === "have-local-offer") {
            console.log(data)
            const pc = this.#callStore.newUserPC(data)
            pc.PeerConectionICE.setRemoteDescription(new RTCSessionDescription(data.answer))
        }
    }

    #reactionAnswerClose(data: object) {
        this.#callStore.peerConnectionUsers.delete(data.idUserTarget)

        console.log(this.#callStore.peerConnectionUsers)

        if (!this.#callStore.peerConnectionUsers.size) {
            this.#callStore.peerConnectionUsers = new Map()

            this.#stream = null
        }
    }

    async #reactiionICECandidate(data: object) {
        if (this.#callStore.newUserPC(data).PeerConectionICE.iceConnectionState == "new" && !this.#callStore.newUserPC(data).isICE) {
            console.log(1111)
            this.#callStore.newUserPC(data).ICEcandidate.forEach(ice => {
                console.log("ОТправленный кандидат: ", ice)
                this.sendSignalICECandidate(ice)
            })

            this.#callStore.newUserPC(data).isICE = true
            console.log(this.#callStore.newUserPC(data).isICE)
        }
        if (data.iceCandidate) {
            try {
                console.log("Полученный кандидат: ", data)
                await this.#callStore.newUserPC(data).PeerConectionICE.addIceCandidate(new RTCIceCandidate(data.iceCandidate));
            } catch (e) {
                console.error('Error adding ICE candidate:', e);
            }
        }
    }

    async #reactionCheckUser(data: object) {
        this.#callStore.newUserPC(data, this.#User.userData, this.#stream)
        console.log("Чек от пользователя: ", data)
        console.log("Объект пользователя: ", this.#callStore.newUserPC(data))
        let offer = await this.#callStore.newUserPC(data).PeerConectionICE.createOffer()
        await this.#callStore.newUserPC(data).PeerConectionICE.setLocalDescription(offer)
        console.log("Отправили оффер: ", data.idUserTarget)
        this.sendSignalAnswer(data.idUserTarget, this.#User.userData?.id, "checkUserActive", "Sent", offer)
    }



    getWebSoketObject(): WebSocket {
        return this.#webSoket
    }

    async startCall() {
        this.#initializationWS()
        console.log("start")
        this.#stream = await navigator.mediaDevices.getUserMedia({ audio: true })
        this.#callStore.statusUser.audio = true
        this.sendSignalStartStream()
        this.sendSignalStatusUser("Active")
    }

    async connectionCall() {
        this.#initializationWS()
        this.#stream = await navigator.mediaDevices.getUserMedia({ audio: true })
        this.#callStore.statusUser.audio = true
        console.log("connect")
        this.sendSignalCheckUser(false)



        this.#callStore.peerConnectionUsers.forEach((el) => {
            console.log(el)
        })



        setTimeout(() => {
            
            this.sendSignalStatusUser("Active")
            
        }, 500)
    }

    stopTrack() {
        this.#callStore.peerConnectionUsers.clear()

        this.#stream.getTracks().forEach((el) => {
            el.stop()
        })

        this.#stream = null
    }


    async micsOnAudio() {
        if (this.#callStore.statusUser?.audio) {
            console.log("Звук так то уже есть!")
        } else {
            const newVideoTrack = await navigator.mediaDevices.getUserMedia({ audio: true })
            this.#callStore.peerConnectionUsers.forEach(async (pc) => {
                pc.PeerConectionICE.addTrack(newVideoTrack.getAudioTracks()[0], this.#stream)
                await this.createOfferRegenerate(pc.PeerConectionICE, pc.idUser)
            })
            this.#callStore.statusUser.audio = true
        }
    }

    async micsOffAudio() {
        if (this.#callStore.statusUser?.audio) {

            let RTC = null
            this.#callStore.peerConnectionUsers.forEach((pc) => {
                console.log(pc)
                pc.PeerConectionICE.getSenders().forEach((rtc) => {
                    if (rtc.track && rtc.track.kind === "audio" && rtc.track.label !== "System Audio") {
                        console.log(rtc)
                        RTC = rtc

                        if (RTC && RTC.track) {
                            let track = RTC.track

                            this.#stream.removeTrack(track)
                            console.log(this.#stream.getTracks())
                            pc.PeerConectionICE.removeTrack(RTC)

                            console.log(pc.PeerConectionICE)
                        }
                    }
                })
            })
            this.#callStore.statusUser.audio = false
            this.sendSignalStatusUser("Active")
        } else {
            console.log("Звук так-то уже выключен!")
        }
    }

    async micsOnVideo() {
        if (this.#callStore.statusUser?.video) {
            console.log("Видео так-то уже включено!")
        } else {
            this.#stream.getVideoTracks().forEach((track) => {
                if (track && track.kind === "video" && track.label.split(":")[0] == "screen") {
                    alert("Трансляция уже идёт, сначала оствновите")
                    return 0
                }
            })
            const newVideoTrack = await navigator.mediaDevices.getUserMedia({ video: true })
            this.#callStore.peerConnectionUsers.forEach((pc) => {
                pc.PeerConectionICE.addTrack(newVideoTrack.getVideoTracks()[0], this.#stream)
                this.createOfferRegenerate(pc.PeerConectionICE, pc.idUser)
            })
            this.#callStore.statusUser.video = true
        }
    }

    async micsOffVideo() {
        if (this.#callStore.statusUser?.video) {

            let RTC = null
            this.#callStore.peerConnectionUsers.forEach((pc) => {
                pc.PeerConectionICE.getSenders().forEach((rtc) => {
                    if (rtc.track && rtc.track.kind === "video" && rtc.track.label.split(":")[0] != "screen") {
                        RTC = rtc

                        if (RTC && RTC.track) {
                            let track = RTC.track

                            this.#stream.removeTrack(track)
                            console.log(this.#stream.getTracks())
                            pc.PeerConectionICE.removeTrack(RTC)

                            console.log(pc.PeerConectionICE)
                        }
                    }
                })
            })
            this.#callStore.statusUser.video = false
            this.sendSignalStatusUser("Active")
        } else {
            console.log("Видео так-то уже отключено!")
        }
    }

    async desktopTranslateOn() {
        if (this.#callStore.statusUser?.viewingStream.status) {
            console.log("Трансляци так-то уже запущена!")
        } else {

            const desktopTranslate = await navigator.mediaDevices.getDisplayMedia({ audio: true, video: true })
            const screenVideoTrack = desktopTranslate.getVideoTracks()[0];




            if (desktopTranslate.getAudioTracks().length) {
                this.#stream.addTrack(desktopTranslate.getAudioTracks()[0])
                this.#stream.getAudioTracks().forEach((el) => {

                    if (el.label === "System Audio") {

                        this.#callStore.peerConnectionUsers.forEach((pc) => {
                            pc.PeerConectionICE.addTrack(desktopTranslate.getAudioTracks()[0], this.#stream)
                        })

                    }
                })

            }

            if (this.#stream.getVideoTracks().length == 1 && desktopTranslate.getVideoTracks().length) {

                this.#stream.removeTrack(this.#stream.getVideoTracks()[0])


                this.#stream.forEach((pc) => {
                    pc.PeerConectionICE.getSenders().forEach((el) => {
                        if (el.kind === "video") {
                            pc.PeerConectionICE.removeTrack(el)
                        }
                    })
                })


                this.#stream.addTrack(screenVideoTrack)

                this.#callStore.peerConnectionUsers.forEach((pc) => {
                    pc.PeerConectionICE.addTrack(screenVideoTrack, this.#stream)
                    this.createOfferRegenerate(pc.PeerConectionICE, pc.idUser)
                })


            } else if (desktopTranslate.getVideoTracks().length) {
                this.#stream.addTrack(screenVideoTrack)

                this.#callStore.peerConnectionUsers.forEach((pc) => {
                    pc.PeerConectionICE.addTrack(screenVideoTrack, this.#stream)
                    this.createOfferRegenerate(pc.PeerConectionICE, pc.idUser)
                })
            }
            this.#callStore.statusUser.viewingStream.status = true
        }

    }

    async desktopTranslateOff() {
        if (this.#callStore.statusUser?.viewingStream.status) {
            this.#callStore.peerConnectionUsers.forEach((pc) => {
                pc.PeerConectionICE.getSenders().forEach((RTC) => {
                    if (RTC.track && RTC.track.kind === "video" && RTC.track.label.split(":")[0] == "screen") {
                        this.#stream.removeTrack(RTC.track)
                        pc.PeerConectionICE.removeTrack(RTC)
                    } else if (RTC.track && RTC.track.kind === "audio" && RTC.track.label === "System Audio") {
                        this.#stream.removeTrack(RTC.track)
                        pc.PeerConectionICE.removeTrack(RTC)
                    }
                })
            })
            this.#callStore.statusUser.viewingStream.status = false
        } else {
            console.log("Трансляци так-то выключена!")
        }

    }

    async muthAudioOn() {
        this.#callStore.statusUser.muth = true
        this.sendSignalStatusUser("Active")
    }

    async muthAudioOff() {
        this.#callStore.statusUser.muth = false
        this.sendSignalStatusUser("Active")
    }

    async closeCall() {
        this.sendSignalStatusUser("Close")

        this.stopTrack()
        this.#callStore.peerConnectionUsers.clear()
        this.#callStore.statusUser.viewingStream.status = false
        this.#callStore.statusUser.audio = false
        this.#callStore.statusUser.video = false

        await this.#webSoket.close()
    }


}
import type { genericRef, User } from "~~/types/other"
import type { StatusMessage } from "~~/types/signaling"


export const callStore = defineStore("Call", () => {

    const user = useUserStore()
    const peerConnectionUsers = ref(new Map)

    const statusUser: genericRef<StatusMessage> = ref({
        type: "Status",
        name: "",
        idUserTarget: "",
        statusUser: null,
        audio: false,
        video: false,
        muth: false,
        idRoom: "",
        Priority: 0,
        viewingStream: {
            status: false,
            idStreamer: ""
        },
        system_option: {}
    })

    const expectationUserCall = ref([])

    const settingCall = ref()

    const activeUserCall = ref([])

    function newUserPC(data: object, userData?: User, track?: MediaStream) {
        console.log(peerConnectionUsers.value.has(data.idUserTarget))
        if (peerConnectionUsers.value.has(data?.idUserTarget)) return peerConnectionUsers.value.get(data.idUserTarget)
        console.log("God")

        const configuration = {
            iceServers: [
                {
                    urls: ['stun:85.208.85.198:3478']
                },
                {
                    urls: 'turn:85.208.85.198:3478',
                    username: user.userData?.userName + user.userData?.id + (new Date(user.userData?.birthday)).getTime(),
                    credential: user.userData?.email + user.userData?.phone
                }
            ]
        }

        const pc = new RTCPeerConnection(configuration)

        const iceCandidate: [string?] = []

        pc.onicecandidate = (event) => {
            console.log(1111)
            if (event.candidate) {
                console.log("Генерируем для: ", data.idUserTarget)
                console.log("Отправляемые ICE", event.candidate)
                iceCandidate.push(JSON.stringify({
                    type: "iceCandidate",
                    idUserTarget: user.userData?.id,
                    idUserAnswer: data.idUserTarget,
                    iceCandidate: event.candidate
                }))
            }
        }

        if (track) {
            track.getTracks().forEach((el) => {
                console.log("Прикрепляемый трек: ", el)
                pc.addTrack(el, track)
            })
        }




        pc.ontrack = (event) => {
            const entry = peerConnectionUsers.value.get(data.idUserTarget);
            if (entry) {
                entry.stream = event.streams[0];
            }
        };

        peerConnectionUsers.value.set(data.idUserTarget, {
            "PeerConectionICE": pc,
            "ICEcandidate": iceCandidate,
            "idUser": data.idUserTarget,
            "PeerConectionMCU": null,
            "stream": null,
            "isICE": false,
            "muth": false
        })

        console.log(peerConnectionUsers)
    }

    return { peerConnectionUsers, expectationUserCall, activeUserCall, newUserPC, settingCall, statusUser }
})
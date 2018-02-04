package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	log "github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
)


var (
	// discordgo session
	discord *discordgo.Session

	// Map of Guild id's to *Play channels, used for queuing and rate-limiting guilds
	queues map[string]chan *Play = make(map[string]chan *Play)
	queuesREST map[string]chan *PlayREST = make(map[string]chan *PlayREST)

	// Sound encoding settings
	BITRATE        = 128
	MAX_QUEUE_SIZE = 6

	// Owner
	OWNER string
	
	// Used to delay subsequent voice chat joins.
	lastRan = time.Now()
	
	// Time to wait between subsequent joins in seconds.
	WAIT = 5
)

// Play represents an individual use of the !airhorn command from chat
type Play struct {
	GuildID   string
	ChannelID string
	UserID    string
	Sound     *Sound

	// The next play to occur after this, only used for chaining sounds like anotha
	Next *Play

	// If true, this was a forced play using a specific airhorn sound name
	Forced bool
}

// PlayREST represents an individual use of the !airhorn command from a RESTful call
type PlayREST struct {
	GuildID   string
	ChannelID string
	Sound     *Sound

	// The next play to occur after this, only used for chaining sounds like anotha
	Next *PlayREST

	// If true, this was a forced play using a specific airhorn sound name
	Forced bool
}


type SoundCollection struct {
	Prefix    string
	Commands  []string
	Sounds    []*Sound
	ChainWith *SoundCollection

	soundRange int
}

// Sound represents a sound clip
type Sound struct {
	Name string

	// Weight adjust how likely it is this song will play, higher = more likely
	Weight int

	// Delay (in milliseconds) for the bot to wait before sending the disconnect request
	PartDelay int

	// Buffer to store encoded PCM packets
	buffer [][]byte
}

// Array of all the sounds we have
var AIRHORN *SoundCollection = &SoundCollection{
	Prefix: "airhorn",
	Commands: []string{
		"!airhorn",
	},
	Sounds: []*Sound{
		createSound("default", 1000, 250),
		createSound("reverb", 800, 250),
		createSound("spam", 800, 0),
		createSound("tripletap", 800, 250),
		createSound("fourtap", 800, 250),
		createSound("distant", 500, 250),
		createSound("echo", 500, 250),
		createSound("clownfull", 250, 250),
		createSound("clownshort", 250, 250),
		createSound("clownspam", 250, 0),
		createSound("highfartlong", 200, 250),
		createSound("highfartshort", 200, 250),
		createSound("midshort", 100, 250),
		createSound("truck", 10, 250),
	},
}

var KHALED *SoundCollection = &SoundCollection{
	Prefix:    "another",
	ChainWith: AIRHORN,
	Commands: []string{
		"!anotha",
		"!anothaone",
	},
	Sounds: []*Sound{
		createSound("one", 1, 250),
		createSound("one_classic", 1, 250),
		createSound("one_echo", 1, 250),
	},
}

var CENA *SoundCollection = &SoundCollection{
	Prefix: "jc",
	Commands: []string{
		"!johncena",
		"!cena",
	},
	Sounds: []*Sound{
		createSound("airhorn", 1, 250),
		createSound("echo", 1, 250),
		createSound("full", 1, 250),
		createSound("jc", 1, 250),
		createSound("nameis", 1, 250),
		createSound("spam", 1, 250),
	},
}

var ETHAN *SoundCollection = &SoundCollection{
	Prefix: "ethan",
	Commands: []string{
		"!ethan",
		"!eb",
		"!ethanbradberry",
		"!h3h3",
	},
	Sounds: []*Sound{
		createSound("areyou_classic", 100, 250),
		createSound("areyou_condensed", 100, 250),
		createSound("areyou_crazy", 100, 250),
		createSound("areyou_ethan", 100, 250),
		createSound("classic", 100, 250),
		createSound("echo", 100, 250),
		createSound("high", 100, 250),
		createSound("slowandlow", 100, 250),
		createSound("cuts", 30, 250),
		createSound("beat", 30, 250),
		createSound("sodiepop", 1, 250),
	},
}

var COW *SoundCollection = &SoundCollection{
	Prefix: "cow",
	Commands: []string{
		"!stan",
		"!stanislav",
	},
	Sounds: []*Sound{
		createSound("herd", 10, 250),
		createSound("moo", 10, 250),
		createSound("x3", 1, 250),
	},
}

var BIRTHDAY *SoundCollection = &SoundCollection{
	Prefix: "birthday",
	Commands: []string{
		"!birthday",
		"!bday",
	},
	Sounds: []*Sound{
		createSound("horn", 50, 250),
		createSound("horn3", 30, 250),
		createSound("sadhorn", 25, 250),
		createSound("weakhorn", 25, 250),
	},
}

var WOW *SoundCollection = &SoundCollection{
	Prefix: "wow",
	Commands: []string{
		"!wowthatscool",
		"!wtc",
	},
	Sounds: []*Sound{
		createSound("thatscool", 50, 250),
	},
}

var JASONBOURNE *SoundCollection = &SoundCollection{
	Prefix: "jasonbourne",
	Commands: []string{
		"!jasonbourne",
		"!jb",
	},
	Sounds: []*Sound{
		createSound("original", 50, 250),
	},
}

var MILK *SoundCollection = &SoundCollection{
	Prefix: "milk",
	Commands: []string{
		"!milk",
	},
	Sounds: []*Sound{
		createSound("original", 50, 250),
	},
}

var AOE1 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!1",
	},
	Sounds: []*Sound{
		createSound("1-yes", 50, 250),
	},
}

var AOE2 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!2",
	},
	Sounds: []*Sound{
		createSound("2-no", 50, 250),
	},
}

var AOE3 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!3",
	},
	Sounds: []*Sound{
		createSound("3-food_please", 50, 250),
	},
}

var AOE4 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!4",
	},
	Sounds: []*Sound{
		createSound("4-wood_please", 50, 250),
	},
}

var AOE5 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!5",
	},
	Sounds: []*Sound{
		createSound("5-gold_please", 50, 250),
	},
}

var AOE6 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!6",
	},
	Sounds: []*Sound{
		createSound("6-stone_please", 50, 250),
	},
}

var AOE7 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!7",
	},
	Sounds: []*Sound{
		createSound("7-ahh", 50, 250),
	},
}

var AOE8 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!8",
	},
	Sounds: []*Sound{
		createSound("8-all_hail", 50, 250),
	},
}

var AOE9 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!9",
	},
	Sounds: []*Sound{
		createSound("9-oooh", 50, 250),
	},
}

var AOE10 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!10",
	},
	Sounds: []*Sound{
		createSound("10-back_to_aoe_1", 50, 250),
	},
}

var AOE11 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!11",
	},
	Sounds: []*Sound{
		createSound("11-herb_laugh", 50, 250),
	},
}

var AOE12 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!12",
	},
	Sounds: []*Sound{
		createSound("12-being_rushed", 50, 250),
	},
}

var AOE13 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!13",
	},
	Sounds: []*Sound{
		createSound("13-blame_your_isp", 50, 250),
	},
}

var AOE14 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!14",
	},
	Sounds: []*Sound{
		createSound("14-start_the_game_already", 50, 250),
	},
}

var AOE15 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!15",
	},
	Sounds: []*Sound{
		createSound("15-dont_point_that_thing_at_me", 50, 250),
	},
}

var AOE16 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!16",
	},
	Sounds: []*Sound{
		createSound("16-enemy_sighted", 50, 250),
	},
}

var AOE17 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!17",
	},
	Sounds: []*Sound{
		createSound("17-it_is_good_to_be_king", 50, 250),
	},
}

var AOE18 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!18",
	},
	Sounds: []*Sound{
		createSound("18-i_need_a_monk", 50, 250),
	},
}

var AOE19 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!19",
	},
	Sounds: []*Sound{
		createSound("19-long_time_no_siege", 50, 250),
	},
}

var AOE20 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!20",
	},
	Sounds: []*Sound{
		createSound("20-my_granny", 50, 250),
	},
}

var AOE21 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!21",
	},
	Sounds: []*Sound{
		createSound("21-nice_town", 50, 250),
	},
}

var AOE22 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!22",
	},
	Sounds: []*Sound{
		createSound("22-quit_touchin_me", 50, 250),
	},
}

var AOE23 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!23",
	},
	Sounds: []*Sound{
		createSound("23-raiding_party", 50, 250),
	},
}

var AOE24 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!24",
	},
	Sounds: []*Sound{
		createSound("24-dad_gum", 50, 250),
	},
}

var AOE25 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!25",
	},
	Sounds: []*Sound{
		createSound("25-smite_me", 50, 250),
	},
}

var AOE26 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!26",
	},
	Sounds: []*Sound{
		createSound("26-the_wonder", 50, 250),
	},
}

var AOE27 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!27",
	},
	Sounds: []*Sound{
		createSound("27-you_play_2_hours", 50, 250),
	},
}

var AOE28 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!28",
	},
	Sounds: []*Sound{
		createSound("28-you_should_see_the_other_guy", 50, 250),
	},
}

var AOE29 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!29",
	},
	Sounds: []*Sound{
		createSound("29-roggan", 50, 250),
	},
}

var AOE30 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!30",
	},
	Sounds: []*Sound{
		createSound("30-wololo", 50, 250),
	},
}

var AOE31 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!31",
	},
	Sounds: []*Sound{
		createSound("31-attack_enemy_now", 50, 250),
	},
}

var AOE32 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!32",
	},
	Sounds: []*Sound{
		createSound("32-extra_villagers", 50, 250),
	},
}

var AOE33 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!33",
	},
	Sounds: []*Sound{
		createSound("33-more_villagers", 50, 250),
	},
}

var AOE34 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!34",
	},
	Sounds: []*Sound{
		createSound("34-build_navy", 50, 250),
	},
}

var AOE35 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!35",
	},
	Sounds: []*Sound{
		createSound("35-stop_build_navy", 50, 250),
	},
}

var AOE36 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!36",
	},
	Sounds: []*Sound{
		createSound("36-wait_for_signal", 50, 250),
	},
}

var AOE37 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!37",
	},
	Sounds: []*Sound{
		createSound("37-build_a_wonder", 50, 250),
	},
}

var AOE38 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!38",
	},
	Sounds: []*Sound{
		createSound("38-gimme_extra_resources", 50, 250),
	},
}

var AOE39 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!39",
	},
	Sounds: []*Sound{
		createSound("39-ally", 50, 250),
	},
}

var AOE40 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!40",
	},
	Sounds: []*Sound{
		createSound("40-enemy", 50, 250),
	},
}

var AOE41 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!41",
	},
	Sounds: []*Sound{
		createSound("41-neutral", 50, 250),
	},
}

var AOE42 *SoundCollection = &SoundCollection{
	Prefix: "aoe2",
	Commands: []string{
		"!42",
	},
	Sounds: []*Sound{
		createSound("42-what_age", 50, 250),
	},
}

var COLLECTIONS []*SoundCollection = []*SoundCollection{
	AIRHORN,
	KHALED,
	CENA,
	ETHAN,
	COW,
	BIRTHDAY,
	WOW,
	JASONBOURNE,
	MILK,
	AOE1,
	AOE2,
	AOE3,
	AOE4,
	AOE5,
	AOE6,
	AOE7,
	AOE8,
	AOE9,
	AOE10,
	AOE11,
	AOE12,
	AOE13,
	AOE14,
	AOE15,
	AOE16,
	AOE17,
	AOE18,
	AOE19,
	AOE20,
	AOE21,
	AOE22,
	AOE23,
	AOE24,
	AOE25,
	AOE26,
	AOE27,
	AOE28,
	AOE29,
	AOE30,
	AOE31,
	AOE32,
	AOE33,
	AOE34,
	AOE35,
	AOE36,
	AOE37,
	AOE38,
	AOE39,
	AOE40,
	AOE41,
	AOE42,
}

// Create a Sound struct
func createSound(Name string, Weight int, PartDelay int) *Sound {
	return &Sound{
		Name:      Name,
		Weight:    Weight,
		PartDelay: PartDelay,
		buffer:    make([][]byte, 0),
	}
}

func (sc *SoundCollection) Load() {
	for _, sound := range sc.Sounds {
		sc.soundRange += sound.Weight
		sound.Load(sc)
	}
}

func (s *SoundCollection) Random() *Sound {
	var (
		i      int
		number int = randomRange(0, s.soundRange)
	)

	for _, sound := range s.Sounds {
		i += sound.Weight

		if number < i {
			return sound
		}
	}
	return nil
}

// Load attempts to load an encoded sound file from disk
// DCA files are pre-computed sound files that are easy to send to Discord.
// If you would like to create your own DCA files, please use:
// https://github.com/nstafie/dca-rs
// eg: dca-rs --raw -i <input wav file> > <output file>
func (s *Sound) Load(c *SoundCollection) error {
	path := fmt.Sprintf("audio/%v_%v.dca", c.Prefix, s.Name)

	file, err := os.Open(path)

	if err != nil {
		fmt.Println("error opening dca file :", err)
		return err
	}

	var opuslen int16

	for {
		// read opus frame length from dca file
		err = binary.Read(file, binary.LittleEndian, &opuslen)

		// If this is the end of the file, just return
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}

		if err != nil {
			fmt.Println("error reading from dca file :", err)
			return err
		}

		// read encoded pcm from dca file
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)

		// Should not be any end of file errors
		if err != nil {
			fmt.Println("error reading from dca file :", err)
			return err
		}

		// append encoded pcm data to the buffer
		s.buffer = append(s.buffer, InBuf)
	}
}

// Plays this sound over the specified VoiceConnection
func (s *Sound) Play(vc *discordgo.VoiceConnection) {
	vc.Speaking(true)
	defer vc.Speaking(false)

	for _, buff := range s.buffer {
		vc.OpusSend <- buff
	}
}

// Attempts to find the current users voice channel inside a given guild
func getCurrentVoiceChannel(user *discordgo.User, guild *discordgo.Guild) *discordgo.Channel {
	for _, vs := range guild.VoiceStates {
		if vs.UserID == user.ID {
			channel, _ := discord.State.Channel(vs.ChannelID)
			return channel
		}
	}
	return nil
}

// Returns a random integer between min and max
func randomRange(min, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return rand.Intn(max-min) + min
}

// Prepares a play
func createPlay(user *discordgo.User, guild *discordgo.Guild, coll *SoundCollection, sound *Sound) *Play {
	// Grab the users voice channel
	channel := getCurrentVoiceChannel(user, guild)
	if channel == nil {
		log.WithFields(log.Fields{
			"user":  user.ID,
			"guild": guild.ID,
		}).Warning("Failed to find channel to play sound in")
		return nil
	}
	
	// Debug - find out what the General channel is.
	log.Info("Channel ID is: " + channel.ID)
	log.Info("Guild ID is: " + guild.ID)

	// Create the play
	play := &Play{
		GuildID:   guild.ID,
		ChannelID: channel.ID,
		UserID:    user.ID,
		Sound:     sound,
		Forced:    true,
	}

	// If we didn't get passed a manual sound, generate a random one
	if play.Sound == nil {
		play.Sound = coll.Random()
		play.Forced = false
	}

	// If the collection is a chained one, set the next sound
	if coll.ChainWith != nil {
		play.Next = &Play{
			GuildID:   play.GuildID,
			ChannelID: play.ChannelID,
			UserID:    play.UserID,
			Sound:     coll.ChainWith.Random(),
			Forced:    play.Forced,
		}
	}

	return play
}

// Prepares a RESTful play
func createPlayREST(coll *SoundCollection, sound *Sound) *PlayREST {
	// Grab the users voice channel
	channel := "354323134192812045"
	guild := "354323134192812043"

	// Create the play
	playREST := &PlayREST{
		GuildID:   guild,
		ChannelID: channel,
		Sound:     sound,
		Forced:    true,
	}

	// If we didn't get passed a manual sound, generate a random one
	if playREST.Sound == nil {
		playREST.Sound = coll.Random()
		playREST.Forced = false
	}

	// If the collection is a chained one, set the next sound
	if coll.ChainWith != nil {
		playREST.Next = &PlayREST{
			GuildID:   playREST.GuildID,
			ChannelID: playREST.ChannelID,
			Sound:     coll.ChainWith.Random(),
			Forced:    playREST.Forced,
		}
	}

	return playREST
}

// Prepares and enqueues a play into the ratelimit/buffer guild queue
func enqueuePlay(user *discordgo.User, guild *discordgo.Guild, coll *SoundCollection, sound *Sound) {
	play := createPlay(user, guild, coll, sound)
	if play == nil {
		return
	}

	// Check if we already have a connection to this guild
	//   yes, this isn't threadsafe, but its "OK" 99% of the time
	_, exists := queues[guild.ID]

	if exists {
		if len(queues[guild.ID]) < MAX_QUEUE_SIZE {
			queues[guild.ID] <- play
		}
	} else {
		queues[guild.ID] = make(chan *Play, MAX_QUEUE_SIZE)
		playSound(play, nil)
	}
}

// Prepares and enqueues a RESTful play into the ratelimit/buffer guild queue
func enqueuePlayREST(coll *SoundCollection, sound *Sound) {
	playREST := createPlayREST(coll, sound)
	if playREST == nil {
		return
	}

	guildID = "354323134192812043"
	
	// Check if we already have a connection to this guild
	//   yes, this isn't threadsafe, but its "OK" 99% of the time
	_, exists := queuesREST[guildID]

	if exists {
		if len(queuesREST[guildID]) < MAX_QUEUE_SIZE {
			queuesREST[guildID] <- playREST
		}
	} else {
		queuesREST[guildID] = make(chan *PlayREST, MAX_QUEUE_SIZE)
		playSoundREST(playREST, nil)
	}
}

// Play a sound
func playSound(play *Play, vc *discordgo.VoiceConnection) (err error) {
	
	log.WithFields(log.Fields{
		"play": play,
	}).Info("Playing sound")
	
	if vc == nil {
		vc, err = discord.ChannelVoiceJoin(play.GuildID, play.ChannelID, false, false)
		log.Info("GuildID: " + play.GuildID + " -- ChannelID: " + play.ChannelID)
		// vc.Receive = false
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Failed to play sound")
			delete(queues, play.GuildID)
			return err
		}
	}

	// If we need to change channels, do that now
	if vc.ChannelID != play.ChannelID {
		vc.ChangeChannel(play.ChannelID, false, false)
		time.Sleep(time.Millisecond * 125)
	}
	
	// Sleep for a specified amount of time before playing the sound
	time.Sleep(time.Millisecond * 32)

	// Play the sound
	play.Sound.Play(vc)
	
	log.Info(len(queues[play.GuildID]))

	// If this is chained, play the chained sound
	if play.Next != nil {
		playSound(play.Next, vc)
	}

	// If there is another song in the queue, recurse and play that
	if len(queues[play.GuildID]) > 0 {
		play := <-queues[play.GuildID]
		playSound(play, vc)
		return nil
	}

	// If the queue is empty, delete it
	time.Sleep(time.Millisecond * time.Duration(play.Sound.PartDelay))
	delete(queues, play.GuildID)
	vc.Disconnect()
	return nil
}

// Play a RESTful sound
func playSoundREST(playREST *PlayREST) (err error) {
	
	log.WithFields(log.Fields{
		"playREST": playREST,
	}).Info("Playing RESTful sound")
	
	vc, err = discord.ChannelVoiceJoin(playREST.GuildID, playREST.ChannelID, false, false)
	
	// vc.Receive = false
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed to play sound")
		delete(queues, play.GuildID)
		return err
	}

	// If we need to change channels, do that now
	if vc.ChannelID != playREST.ChannelID {
		vc.ChangeChannel(playREST.ChannelID, false, false)
		time.Sleep(time.Millisecond * 125)
	}
	
	// Sleep for a specified amount of time before playing the sound
	time.Sleep(time.Millisecond * 32)

	// Play the sound
	playREST.Sound.Play(vc)
	
	log.Info(len(queuesREST[play.GuildID]))

	// If this is chained, play the chained sound
	if playREST.Next != nil {
		playSoundREST(playREST.Next, vc)
	}

	// If there is another song in the queue, recurse and play that
	if len(queuesREST[play.GuildID]) > 0 {
		playREST := <-queuesREST[play.GuildID]
		playSoundREST(playREST, vc)
		return nil
	}

	// If the queue is empty, delete it
	time.Sleep(time.Millisecond * time.Duration(play.Sound.PartDelay))
	delete(queuesREST, playREST.GuildID)
	vc.Disconnect()
	return nil
}

func onReady(s *discordgo.Session, event *discordgo.Ready) {
	log.Info("Received READY payload")
	s.UpdateStatus(0, "AoEII")
}

func scontains(key string, options ...string) bool {
	for _, item := range options {
		if item == key {
			return true
		}
	}
	return false
}

func calculateAirhornsPerSecond(cid string) {
	current, _ := strconv.Atoi(rcli.Get("airhorn:a:total").Val())
	time.Sleep(time.Second * 10)
	latest, _ := strconv.Atoi(rcli.Get("airhorn:a:total").Val())

	discord.ChannelMessageSend(cid, fmt.Sprintf("Current APS: %v", (float64(latest-current))/10.0))
}

func displayBotStats(cid string) {
	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)

	users := 0
	for _, guild := range discord.State.Ready.Guilds {
		users += len(guild.Members)
	}

	w := &tabwriter.Writer{}
	buf := &bytes.Buffer{}

	w.Init(buf, 0, 4, 0, ' ', 0)
	fmt.Fprintf(w, "```\n")
	fmt.Fprintf(w, "Discordgo: \t%s\n", discordgo.VERSION)
	fmt.Fprintf(w, "Go: \t%s\n", runtime.Version())
	fmt.Fprintf(w, "Memory: \t%s / %s (%s total allocated)\n", humanize.Bytes(stats.Alloc), humanize.Bytes(stats.Sys), humanize.Bytes(stats.TotalAlloc))
	fmt.Fprintf(w, "Tasks: \t%d\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "Servers: \t%d\n", len(discord.State.Ready.Guilds))
	fmt.Fprintf(w, "Users: \t%d\n", users)
	fmt.Fprintf(w, "```\n")
	w.Flush()
	discord.ChannelMessageSend(cid, buf.String())
}

func displayUserStats(cid, uid string) {
	keys, err := rcli.Keys(fmt.Sprintf("airhorn:*:user:%s:sound:*", uid)).Result()
	if err != nil {
		return
	}

	totalAirhorns := utilSumRedisKeys(keys)
	discord.ChannelMessageSend(cid, fmt.Sprintf("Total Airhorns: %v", totalAirhorns))
}

func displayServerStats(cid, sid string) {
	keys, err := rcli.Keys(fmt.Sprintf("airhorn:*:guild:%s:sound:*", sid)).Result()
	if err != nil {
		return
	}

	totalAirhorns := utilSumRedisKeys(keys)
	discord.ChannelMessageSend(cid, fmt.Sprintf("Total Airhorns: %v", totalAirhorns))
}

func utilGetMentioned(s *discordgo.Session, m *discordgo.MessageCreate) *discordgo.User {
	for _, mention := range m.Mentions {
		if mention.ID != s.State.Ready.User.ID {
			return mention
		}
	}
	return nil
}

// Handles bot operator messages, should be refactored (lmao)
func handleBotControlMessages(s *discordgo.Session, m *discordgo.MessageCreate, parts []string, g *discordgo.Guild) {
	if scontains(parts[1], "status") {
		displayBotStats(m.ChannelID)
	} else if scontains(parts[1], "stats") {
		if len(m.Mentions) >= 2 {
			displayUserStats(m.ChannelID, utilGetMentioned(s, m).ID)
		} else if len(parts) >= 3 {
			displayUserStats(m.ChannelID, parts[2])
		} else {
			displayServerStats(m.ChannelID, g.ID)
		}
	} else if scontains(parts[1], "aps") {
		s.ChannelMessageSend(m.ChannelID, ":ok_hand: give me a sec m8")
		go calculateAirhornsPerSecond(m.ChannelID)
	}
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if len(m.Content) <= 0 || (m.Content[0] != '!' && len(m.Mentions) < 1) {
		return
	}

	msg := strings.Replace(m.ContentWithMentionsReplaced(), s.State.Ready.User.Username, "username", 1)
	parts := strings.Split(strings.ToLower(msg), " ")

	channel, _ := discord.State.Channel(m.ChannelID)
	if channel == nil {
		log.WithFields(log.Fields{
			"channel": m.ChannelID,
			"message": m.ID,
		}).Warning("Failed to grab channel")
		return
	}

	guild, _ := discord.State.Guild(channel.GuildID)
	if guild == nil {
		log.WithFields(log.Fields{
			"guild":   channel.GuildID,
			"channel": channel,
			"message": m.ID,
		}).Warning("Failed to grab guild")
		return
	}

	// If this is a mention, it should come from the owner (otherwise we don't care)
	if len(m.Mentions) > 0 && m.Author.ID == OWNER && len(parts) > 0 {
		mentioned := false
		for _, mention := range m.Mentions {
			mentioned = (mention.ID == s.State.Ready.User.ID)
			if mentioned {
				break
			}
		}

		if mentioned {
			handleBotControlMessages(s, m, parts, guild)
		}
		return
	}

	// Find the collection for the command we got
	for _, coll := range COLLECTIONS {
		if scontains(parts[0], coll.Commands...) {

			// If they passed a specific sound effect, find and select that (otherwise play nothing)
			var sound *Sound
			if len(parts) > 1 {
				for _, s := range coll.Sounds {
					if parts[1] == s.Name {
						sound = s
					}
				}

				if sound == nil {
					return
				}
			}

			go enqueuePlay(m.Author, guild, coll, sound)
			return
		}
	}
}

func onMessageCreateREST(sid string) {
	if len((sid) <= 0) {
		return
	}

	parts := strings.Split(strings.ToLower(sid), " ")

	// Find the collection for the command we got
	for _, coll := range COLLECTIONS {
		if scontains(parts[0], coll.Commands...) {

			// If they passed a specific sound effect, find and select that (otherwise play nothing)
			var sound *Sound
			if len(parts) > 1 {
				for _, s := range coll.Sounds {
					if parts[1] == s.Name {
						sound = s
					}
				}

				if sound == nil {
					return
				}
			}

			go enqueuePlayREST(coll, sound)
			return
		}
	}
}

func playSoundREST(w http.ResponseWriter, r *http.Request) {
        params := mux.Vars(r)
        log.Info("RESTful request to play sound '" + params["id"] + "'")
		onMessageCreateREST(params["id"])
		return
}

func main() {
	var (
		Token      = flag.String("t", "", "Discord Authentication Token")
		Shard      = flag.String("s", "", "Shard ID")
		ShardCount = flag.String("c", "", "Number of shards")
		Owner      = flag.String("o", "", "Owner ID")
		err        error
	)
	flag.Parse()

	if *Owner != "" {
		OWNER = *Owner
	}

	// Preload all the sounds
	log.Info("Preloading sounds...")
	for _, coll := range COLLECTIONS {
		coll.Load()
	}

	// Create a discord session
	log.Info("Starting discord session...")
	discord, err = discordgo.New(*Token)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Failed to create discord session")
		return
	}

	// Set sharding info
	discord.ShardID, _ = strconv.Atoi(*Shard)
	discord.ShardCount, _ = strconv.Atoi(*ShardCount)

	if discord.ShardCount <= 0 {
		discord.ShardCount = 1
	}

	discord.AddHandler(onReady)
	discord.AddHandler(onMessageCreate)

	err = discord.Open()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Failed to create discord websocket connection")
		return
	}

	// Launch RESTful API
	router := mux.NewRouter()
    router.HandleFunc("/airhorn/{id}", playSoundREST).Methods("GET")
	
	// We're running!
	log.Info("CLANSPBOT is ready to fuck it up.")
	
	// Launch synchronous HTTP service.
    log.Fatal(http.ListenAndServe(":8000", router))
	
	// Wait for a signal to quit
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c
}

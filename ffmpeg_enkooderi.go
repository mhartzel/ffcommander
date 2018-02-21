package main

import (
	"fmt"
	"os/exec"
	"os"
	"strings"
	"flag"
	"log"
	"strconv"
	"path/filepath"
)

// Global variable definitions
type Video_data struct {
	audio_codec string
	sample_rate int
	number_of_channels int
	video_codec string
	vertical_resolution int
	horizontal_resolution int
	aspect_ratio string
	commandline []string
}


func run_external_command(command_to_run_str_slice []string) ([]string,  error) {

	var unsorted_ffprobe_information_str_slice []string
	command_output_str := ""

	// Create the struct needed for running the external command
	command_struct := exec.Command(command_to_run_str_slice[0], command_to_run_str_slice[1:]...)

	// Run external command
	command_output, error_message := command_struct.CombinedOutput()

	command_output_str = string(command_output) // The output of the command is []byte convert it to a string

	// Split the output of the command to lines and store in a slice
	for _, line := range strings.Split(command_output_str, "\n")  {
		unsorted_ffprobe_information_str_slice = append(unsorted_ffprobe_information_str_slice, line)
	}

	return unsorted_ffprobe_information_str_slice, error_message
}

func sort_raw_ffprobe_information(unsorted_ffprobe_information_str_slice []string) (Video_data) {

	// Parse ffprobe output, find wrapper, video- and audiostream information in it,
        // and store this info in stream specific maps

	var complete_stream_info_map = make(map[int][]string)
	var video_stream_info_map = make(map[string]string)
	var audio_stream_info_map = make(map[string]string)
	var wrapper_info_map = make(map[string]string)

	var stream_info_str_slice []string
	var text_line_str_slice []string
	var wrapper_info_str_slice []string

	var stream_number_int int
	var stream_data_str string
	var string_to_remove_str_slice []string
	var error error // a variable named error of type error

	// Collect information about all streams in the media file.
        // The info is collected to stream specific slices and stored in map: complete_stream_info_map
        // The stream number is used as the map key when saving info slice to map

	for _,text_line := range unsorted_ffprobe_information_str_slice {
		stream_number_int = -1
		stream_info_str_slice = nil

		// If there are many programs in the file, then the stream information is listed twice by ffprobe,
                // discard duplicate data.
		if strings.HasPrefix(text_line, "programs.program"){
			continue
		}

		if strings.HasPrefix(text_line, "streams.stream") {
			text_line_str_slice = strings.Split(strings.Replace(text_line, "streams.stream.","",1),".")

			// Convert stream number from string to int
			error = nil

			if stream_number_int, error = strconv.Atoi(text_line_str_slice[0]) ; error != nil {
				// Stream number could not be understood, skip the stream
				continue
			}

			string_to_remove_str_slice = string_to_remove_str_slice[:0] // Empty the slice so that allocated slice ram space remains and is not garbage collected.
			string_to_remove_str_slice = append(string_to_remove_str_slice, "streams.stream.",strconv.Itoa(stream_number_int),".")
			stream_data_str = strings.Replace(text_line, strings.Join(string_to_remove_str_slice,""),"",1) // Remove the unwanted string in front of the text line.
			stream_data_str = strings.Replace(stream_data_str, "\"", "", -1) // Remove " characters from the data.

			// Add found stream info line to a slice of previously stored info
			// and store it in a map. The stream number acts as the map key.
			if _, item_found := complete_stream_info_map[stream_number_int] ; item_found == true {
				stream_info_str_slice = complete_stream_info_map[stream_number_int]
			}
			stream_info_str_slice = append(stream_info_str_slice, stream_data_str)
			complete_stream_info_map[stream_number_int] = stream_info_str_slice
		}
		// Get media file wrapper information and store it in a slice.
		if strings.HasPrefix(text_line, "format") {
			wrapper_info_str_slice = strings.Split(strings.Replace(text_line, "format.", "", 1), "=")
			wrapper_key := strings.TrimSpace(wrapper_info_str_slice[0])
			wrapper_value := strings.TrimSpace(strings.Replace(wrapper_info_str_slice[1],"\"", "", -1)) // Remove whitespace and " charcters from the data
			wrapper_info_map[wrapper_key] = wrapper_value
		}
	}

	// Find video and audio stream information and store it as key value pairs in video_stream_info_map and audio_stream_info_map.
	// Discard streams that are not audio or video
	var stream_type_is_video bool = false
	var stream_type_is_audio bool = false

	for _, stream_info_str_slice := range complete_stream_info_map {

		stream_type_is_video = false
		stream_type_is_audio = false

		for _, text_line := range stream_info_str_slice {

			if strings.Contains(text_line, "codec_type=video") {
				stream_type_is_video = true
			}

		}

		for _, text_line := range stream_info_str_slice {

			if strings.Contains(text_line, "codec_type=audio") {
				stream_type_is_audio = true
			}

		}

		if stream_type_is_video == true {

			for _, text_line := range stream_info_str_slice {

				temp_slice := strings.Split(text_line, "=")
				video_key := strings.TrimSpace(temp_slice[0])
				video_value := strings.TrimSpace(temp_slice[1])
				video_stream_info_map[video_key] = video_value
				}

		}

		if stream_type_is_audio == true {
			for _, text_line := range stream_info_str_slice {

				temp_slice := strings.Split(text_line, "=")
				audio_key := strings.TrimSpace(temp_slice[0])
				audio_value := strings.TrimSpace(temp_slice[1])
				audio_stream_info_map[audio_key] = audio_value
				}

		}

	}

	// Find specific video and audio info we need and store in a struct that we return to the main program.
	var video_info_struct Video_data

	video_info_struct.audio_codec = audio_stream_info_map["codec_name"]
	video_info_struct.sample_rate,_ = strconv.Atoi(audio_stream_info_map["sample_rate"])
	video_info_struct.number_of_channels,_ = strconv.Atoi(audio_stream_info_map["channels"])
	video_info_struct.video_codec = video_stream_info_map["codec_name"]
	video_info_struct.vertical_resolution,_ = strconv.Atoi(video_stream_info_map["width"])
	video_info_struct.horizontal_resolution,_ = strconv.Atoi(video_stream_info_map["height"])
	video_info_struct.aspect_ratio = video_stream_info_map["display_aspect_ratio"]

	return(video_info_struct)
}


func main() {

	// FIXME
	// wrapper_info_mapin tieto kerätään mutta sitä ei käytetä mitenkään.

	// Define commandline options
	var no_deinterlace_bool = flag.Bool("nd", false, "No Deinterlace")
	var subtitle_int = flag.Int("s", -1, "Subtitle `number`")
	var grayscale_bool = flag.Bool("gr", false, "Grayscale")
	var denoise_bool = flag.Bool("dn", false, "Denoise")
	var force_stereo_bool = flag.Bool("st", false, "Force Audio To Stereo")
	var autocrop_bool = flag.Bool("ac", false, "Autocrop")
	var force_hd_bool = flag.Bool("hd", false, "Force Video To HD, Profile = High, Level = 4.1, Bitrate = 8000k")
	var input_filenames []string

	// Parse commandline options
	flag.Parse()

	compression_options_sd := []string{"-c:v", "libx264", "-preset", "medium", "-profile", "main", "-level", "4.0", "-b:v", "1600k"}
	compression_options_hd := []string{"-c:v", "libx264", "-preset", "medium", "-profile", "high", "-level", "4.1", "-b:v", "8000k"}
	compression_pass_1_options := []string{"-pass", "1", "-acodec", "copy", "-f", "mp4", "/dev/null"}
	compression_pass_2_options := []string{"-pass", "2", "-acodec", "copy", "-f", "mp4"}
	denoise_options := []string{"hqdn3d=3.0:3.0:2.0:3.0"}
	deinterlace_options := []string{"idet,yadif=0:deint=interlaced"}
	ffmpeg_commandline_start := []string{"ffmpeg", "-y", "-loglevel", "8", "-threads", "auto"}
	subtitle_number := *subtitle_int
	grayscale_options := []string{"lut=u=128:v=128"}
	var crop_options []string
	subtitle_options := ""
	output_directory_name := "00-valmiit"

	var ffmpeg_commandline []string

	fmt.Println()
	fmt.Println("compression_options_sd:",compression_options_sd)
	fmt.Println("compression_options_hd:",compression_options_hd)
	fmt.Println("compression_pass_1_options:",compression_pass_1_options)
	fmt.Println("compression_pass_2_options:",compression_pass_2_options)
	fmt.Println("denoise_options:",denoise_options)
	fmt.Println("deinterlace_options:",deinterlace_options)
	fmt.Println("ffmpeg_commandline_start:",ffmpeg_commandline_start)
	fmt.Println("subtitle_number:",subtitle_number)
	fmt.Println("grayscale_options:",grayscale_options)
	fmt.Println("subtitle_options:",subtitle_options)
	fmt.Println("crop_options",crop_options)
	fmt.Println()

	// The unparsed options left on the commandline are filenames, store them in a slice.
	for _,file_name := range flag.Args()  {
		input_filenames = append(input_filenames, file_name)
	}

	// start_options_for_the_filter := "-vf "
	// decomb_options_string := "idet,yadif=0:deint=interlaced"
	// denoise_options_string := ",hqdn3d=3.0:3.0:2.0:3.0"


	// FIXME
	fmt.Println(*autocrop_bool, *grayscale_bool, *subtitle_int, *no_deinterlace_bool, *denoise_bool, *force_stereo_bool, *force_hd_bool)
	fmt.Println("\nSlice:", input_filenames)
	fmt.Println("\n")


	for _,file_name := range input_filenames {

		var command_to_run_str_slice []string

		// Split filename to path + filename
		inputfile_absolute_path,_ := filepath.Abs(file_name)
		inputfile_path := filepath.Dir(inputfile_absolute_path)
		inputfile_name := filepath.Base(file_name)
		output_filename_extension := filepath.Ext(inputfile_name)
		output_file_absolute_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, output_filename_extension) + ".mp4")

		// If output directory does not exist in the input file path then create it.
		if _, err := os.Stat(filepath.Join(inputfile_path, output_directory_name)); os.IsNotExist(err) {
			os.Mkdir(filepath.Join(inputfile_path, output_directory_name), 0777)
		}

		// FIXME
		fmt.Println("inputfile_path:", inputfile_path)
		fmt.Println("inputfile_name:", inputfile_name)
		fmt.Println("output_file_absolute_path:", output_file_absolute_path)

		command_to_run_str_slice = append(command_to_run_str_slice, "ffprobe","-loglevel","16","-show_entries","format:stream","-print_format","flat","-i")
		command_to_run_str_slice = append(command_to_run_str_slice, file_name)

		unsorted_ffprobe_information_str_slice, error_message := run_external_command(command_to_run_str_slice)

		if error_message != nil {
			log.Fatal(error_message)
		}

		video_info_struct := sort_raw_ffprobe_information(unsorted_ffprobe_information_str_slice)

		// FIXME
		fmt.Println(file_name)
		fmt.Println("video_info_struct.audio_codec:", video_info_struct.audio_codec)
		fmt.Println("video_info_struct.sample_rate:", video_info_struct.sample_rate)
		fmt.Println("video_info_struct.number_of_channels:", video_info_struct.number_of_channels)
		fmt.Println("video_info_struct.video_codec:", video_info_struct.video_codec)
		fmt.Println("video_info_struct.vertical_resolution:", video_info_struct.vertical_resolution)
		fmt.Println("video_info_struct.horizontal_resolution:", video_info_struct.horizontal_resolution)
		fmt.Println("video_info_struct.aspect_ratio:", video_info_struct.aspect_ratio)
		fmt.Println("video_info_struct.commandline:", video_info_struct.commandline)
		fmt.Println()
		fmt.Println("autocrop_bool:", *autocrop_bool)

		if *autocrop_bool == true {
			autocrop_settings_slice := []string{"ffmpeg", "-t", "1800", "-i", file_name, "-sn", "-f", "matroska", "-an", "-vf", "cropdetect=24:16:0", "-y", "-crf", "51", "-preset", "ultrafast", "/dev/null", "2>&1", "|", "grep", "-o", "crop=.*", "|", "sort", "-bh", "|", "uniq", "-c", "|", "sort", "-bh", "|", "tail", "-n1", "|", "grep", "-o", "crop.*"}

			// FIXME
			println("autocrop_settings_slice:")

			for _,item := range autocrop_settings_slice {
				print(item, " ")
			}

			// FIXME
			println()
			println()
		}

		// Create the start of ffmpeg commandline
		ffmpeg_commandline = append(ffmpeg_commandline, ffmpeg_commandline_start...)
		ffmpeg_commandline = append(ffmpeg_commandline, "-i", file_name)

		ffmpeg_filter_options := ""

		// If there is no subtitles to overlay on video use simple video filter processing in ffmpeg
		if subtitle_number == -1 {
			ffmpeg_commandline = append(ffmpeg_commandline, "-vf")

			// Add deinterlace commands to ffmpeg commandline
			if *no_deinterlace_bool == false {
				ffmpeg_filter_options = ffmpeg_filter_options + strings.Join(deinterlace_options, "")
			}

			// FIXME mihkäs tää croppi kuuluu ?
			// Add crop options to ffmpeg commandline
			if len(crop_options) > 0 {
				if ffmpeg_filter_options != "" {
					ffmpeg_filter_options = ffmpeg_filter_options + ","
				}
				ffmpeg_filter_options = ffmpeg_filter_options + strings.Join(crop_options, "")
			}
			// Add denoise options to ffmpeg commandline
			if *denoise_bool == true {
				if ffmpeg_filter_options != "" {
					ffmpeg_filter_options = ffmpeg_filter_options + ","
				}
				ffmpeg_filter_options = ffmpeg_filter_options + strings.Join(denoise_options, "")
			}

			// Add grayscale options to ffmpeg commandline
			if *grayscale_bool == true {
				if ffmpeg_filter_options != "" {
					ffmpeg_filter_options =  ffmpeg_filter_options + ","
				}
				ffmpeg_filter_options = ffmpeg_filter_options + strings.Join(grayscale_options, "")
			}
		}

		// Add video filter options to ffmpeg commanline
		ffmpeg_commandline = append(ffmpeg_commandline, ffmpeg_filter_options)

		// If video horizontal resolution is over 700 pixel choose HD video compression settings
		compression_options := compression_options_sd

		if *force_hd_bool || video_info_struct.horizontal_resolution > 700 {
			compression_options = compression_options_hd
		}

		// Add compression options to ffmpeg commandline
		ffmpeg_commandline = append(ffmpeg_commandline, compression_options...)


		// Add outfile path to ffmpeg commandline
		ffmpeg_commandline = append(ffmpeg_commandline, output_file_absolute_path)

		// FIXME
		fmt.Println("ffmpeg_commandline:", ffmpeg_commandline)

		run_external_command(ffmpeg_commandline)

// -passlogfile[:stream_specifier] prefix (output,per-stream)
// Set two-pass log file name prefix to prefix, the default file name prefix is ``ffmpeg2pass''. The complete file name will be PREFIX-N.log, where N is a number specific to the output stream


// Ilman optioita, Denoise päällä:
//
// Pass 1 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -vf idet,yadif=0:deint=interlaced,hqdn3d=3.0:3.0:2.0:3.0 -c:v libx264 -preset medium -profile main -level 4.0 -b:v 1600k -pass 1 -acodec copy -f mp4 /dev/null
// Pass 2 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -vf idet,yadif=0:deint=interlaced,hqdn3d=3.0:3.0:2.0:3.0 -c:v libx264 -preset medium -profile main -level 4.0 -b:v 1600k -pass 2 -acodec copy -f mp4 FileOut

// AUTOCROP - KOMENTO:
// ffmpeg -t 1800 -i FileIn -sn -f matroska -an -vf cropdetect=24:16:0 -y -crf 51 -preset ultrafast /dev/null 2>&1 | grep -o crop=.* | sort -bh | uniq -c | sort -bh | tail -n1 | grep -o crop.*

// Subtitle nro 2 + Denoise:
//
// Pass 1 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -filter_complex " [0:s:2] copy [kropattu_tekstitys] ; [0:v:0] idet,yadif=0:deint=interlaced,hqdn3d=3.0:3.0:2.0:3.0 [kropattu_video] ; [kropattu_video] [kropattu_tekstitys] overlay=main_w-overlay_w-0:main_h-overlay_h+55 [valmis_kropattu_video]" -map [valmis_kropattu_video] -c:v libx264 -preset medium -profile main -level 4.0 -b:v 1600k -pass 1 -map 0:a:0 -acodec copy -f mp4 /dev/null
// Pass 2 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -filter_complex " [0:s:2] copy [kropattu_tekstitys] ; [0:v:0] idet,yadif=0:deint=interlaced,hqdn3d=3.0:3.0:2.0:3.0 [kropattu_video] ; [kropattu_video] [kropattu_tekstitys] overlay=main_w-overlay_w-0:main_h-overlay_h+55 [valmis_kropattu_video]" -map [valmis_kropattu_video] -c:v libx264 -preset medium -profile main -level 4.0 -b:v 1600k -pass 2 -map 0:a:0 -acodec copy -f mp4 FileOut
//
// Subtitle + Deinterlace + Denoise + Grayscale
//
// Pass 1 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -filter_complex " [0:s:7] copy [kropattu_tekstitys] ; [0:v:0] idet,yadif=0:deint=interlaced,hqdn3d=3.0:3.0:2.0:3.0,lut=u=128:v=128 [kropattu_video] ; [kropattu_video] [kropattu_tekstitys] overlay=main_w-overlay_w-0:main_h-overlay_h+55 [valmis_kropattu_video]" -map [valmis_kropattu_video] -c:v libx264 -preset medium -profile main -level 4.0 -b:v 1600k -pass 1 -map 0:a:0 -acodec copy -f mp4 /dev/null
// Pass 2 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -filter_complex " [0:s:7] copy [kropattu_tekstitys] ; [0:v:0] idet,yadif=0:deint=interlaced,hqdn3d=3.0:3.0:2.0:3.0,lut=u=128:v=128 [kropattu_video] ; [kropattu_video] [kropattu_tekstitys] overlay=main_w-overlay_w-0:main_h-overlay_h+55 [valmis_kropattu_video]" -map [valmis_kropattu_video] -c:v libx264 -preset medium -profile main -level 4.0 -b:v 1600k -pass 2 -map 0:a:0 -acodec copy -f mp4 FileOut
//
// ##########################################
// # Filttereiden komentorivin rakentaminen #
// ##########################################
// # filttereillä luodaan 2 erillistä videon käsittelyputkea, toisessa käsitellään video ja toisessa tekstitys.
// # Erillisellä käsittelyllä saadaan teksti pysymään videon päällä, vaikka alla olevaa videota pienennetään (kropataan).
// # Käsitellyt kuvastreamit yhdistetään sitten overlay - komennolla.
// # Käsittelyputki 1 järjestys: subtitle, crop
// # Käsittelyputki 2 järjestys: decomb, crop, denoise, grayscale
// # audiodelay
// #
// # -filter_complex "[0:s:10] crop=1920:816:1920-0:1080-816 [kropattu_tekstitys] ; [0:v:0] idet,yadif=0:deint=interlaced,crop=1920:816:0:132 [kropattu_video] ; [kropattu_video] [kropattu_tekstitys] overlay [valmis_kropattu_video]"  -map [valmis_kropattu_video] -c:v libx264 


// HD + Denoise:
//
// Pass 1 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -vf idet,yadif=0:deint=interlaced,hqdn3d=3.0:3.0:2.0:3.0 -c:v libx264 -preset medium -profile high -level 4.1 -b:v 8000k -pass 1 -acodec copy -f mp4 /dev/null
// Pass 2 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -vf idet,yadif=0:deint=interlaced,hqdn3d=3.0:3.0:2.0:3.0 -c:v libx264 -preset medium -profile high -level 4.1 -b:v 8000k -pass 2 -acodec copy -f mp4 FileOut

// No Deinterlace + Denoise:
//
// Pass 1 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -vf ,hqdn3d=3.0:3.0:2.0:3.0 -c:v libx264 -preset medium -profile main -level 4.0 -b:v 1600k -pass 1 -acodec copy -f mp4 /dev/null
// Pass 2 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -vf ,hqdn3d=3.0:3.0:2.0:3.0 -c:v libx264 -preset medium -profile main -level 4.0 -b:v 1600k -pass 2 -acodec copy -f mp4 FileOut

// Grayscale + Denoise:
//
// Pass 1 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -vf idet,yadif=0:deint=interlaced,hqdn3d=3.0:3.0:2.0:3.0,lut=u=128:v=128 -c:v libx264 -preset medium -profile main -level 4.0 -b:v 1600k -pass 1 -acodec copy -f mp4 /dev/null
// Pass 2 Pakkauskomento: ffmpeg -y -loglevel 8 -threads auto -i FileIn -vf idet,yadif=0:deint=interlaced,hqdn3d=3.0:3.0:2.0:3.0,lut=u=128:v=128 -c:v libx264 -preset medium -profile main -level 4.0 -b:v 1600k -pass 2 -acodec copy -f mp4 FileOut







                // # Pakkaus Pass 1
                // echo
                // echo "Pass 1 Pakkauskomento: "$PAKKAUSKOMENTO1

                // bash -c "ffmpeg -y $LOGLEVEL $THREADS -i "$LAHDETIEDOSTO" $FILTTEREIDEN_KOMENNOT $FRAMERATE -c:v libx264 -preset $PRESET -profile $PROFILE -level $LEVEL -b:v $BITRATE -pass 1 $AUDIO_MAP_ASETUKSET $AUDIO_DELAY_COMMAND $AUDION_ASETUKSET -f mp4 /dev/null"
                // # "$PAKKAUSKOMENTO1"
                // if [ "$?" -ne "0"  ] ; then echo ; echo "Pass 1 pakkauskomento tuotti virheen" ; echo ; exit 1 ; fi
                // echo
                // echo "Pass 2 Pakkauskomento: "$PAKKAUSKOMENTO2

                // # Pakkaus Pass 2
                // bash -c "ffmpeg -y $LOGLEVEL $THREADS -i "$LAHDETIEDOSTO" $FILTTEREIDEN_KOMENNOT $FRAMERATE -c:v libx264 -preset $PRESET -profile $PROFILE -level $LEVEL -b:v $BITRATE -pass 2 $AUDIO_MAP_ASETUKSET $AUDIO_DELAY_COMMAND $AUDION_ASETUKSET -f mp4 "$KOHDEPOLKU"/"$KOHDETIEDOSTO""
                // # "$PAKKAUSKOMENTO2"
                // if [ "$?" -ne "0"  ] ; then echo ; echo "Pass 2 pakkauskomento tuotti virheen" ; echo ; exit 1 ; fi
                // echo
                // echo "###############################################################################"
                // echo "# Tiedosto "$KOHDETIEDOSTO" on valmis :)"
                // echo "# Se löytyy hakemistosta: "$KOHDEPOLKU
                // echo "###############################################################################"
                // echo

		// // FIXME
		// fmt.Println(file_name, "complete_stream_info_map:", "\n")
		// // for item, stream_info_str_slice := range complete_stream_info_map {
		// for key, stream_info_str_slice := range complete_stream_info_map {
		// 	fmt.Println("\n")
		// 	fmt.Println("key:", key)
		// 	fmt.Println("-----------------------------------")
		// 	// fmt.Println("stream_info_str_slice:", stream_info_str_slice)
		// 	for _,value := range stream_info_str_slice {
		// 		fmt.Println(value)
		// 	}
		// 	// fmt.Println(item, " = ", complete_stream_info_map[item], "\n")
		// }
		// fmt.Println("\n")
		// fmt.Println("Wrapper info:")
		// fmt.Println("-------------")

		// for item := range wrapper_info_map {
		// 	fmt.Println(item, "=", wrapper_info_map[item])
		// }
		// fmt.Println()

		// fmt.Println("video_stream_info_map:")
		// fmt.Println("-----------------------")

		// for item := range video_stream_info_map {
		// 	fmt.Println(item, "=", video_stream_info_map[item])
		// }
		// fmt.Println()

		// fmt.Println("audio_stream_info_map:")
		// fmt.Println("-----------------------")

		// for item := range audio_stream_info_map {
		// 	fmt.Println(item, "=", audio_stream_info_map[item])
		// }
		// fmt.Println()

	}
}

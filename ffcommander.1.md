% FFCOMMANDER(1) ffcommander 2.35
% Mikael Hartzell (C) 2018
% 2021

# Name
ffcommander - An easy frontend to FFmpeg and Imagemagick to automatically process video and manipulate subtitles.

# Description
I wrote FFcommander to replace Handbrake in my workflow and because of this I made FFcomander do all the things I usually did with Handbrake. I also always want to process similar files in a similar fashion and made FFcommander be as automatic as it can be doing something. This resulted in the following defaults (which can also be overridden with commandline options):  
- Always deinterlace.  
- Calculate video bitrate automatically based on video size.  
- Always use 2-pass video encoding to get the best quality in the smallest possible file size. Constant bitrate compression is also available but in my opinion it's quality is not as good. Nothing beats 2-pass quality in dark scenes.   
- Always copy original audio as it is to the processed video.  

# What this program exels at
In my opinion the -sp -sr features are something I use all the time and have not seen in any other program. These options let you burn subtitles on top of video while resizing and moving them automatically up or down right at the edge of the screen. This prevents subtitles ever being displayed on top of a actors face.

You can also cut parts of a longer video and create a compilation of these (option -sf).

Create an HD and SD - version of a video at the same time.

Burn timecode on top of video. Autocrop or change video to grayscale, denoise ior inverse telecine video.

Burn subtitles on top of video while converting them to grayscale. Mux DVD or Bluray subtitle images into the processed file. This lets you turn subtitles on or off while watching the video.

Scan source files and display video, audio and subtitles info to find files that you can all process with the same options in one go.

Display the complex commandlines that FFcommander creates for FFmpeg to learn how FFmpeg works.

# Installation
FFcommander source code does not have any external dependencies so building the program is very easy. You just need to install a couple of programs from your distros repo.  

Install FFmpeg, Imagemagick, git, and the go - language compiler:  
Arch or Manjaro: **pacman -S ffmpeg imagemagick go go-tools**  
Debian or Ubuntu: **apt-get install ffmpeg imagemagick golang**  
Get the source code: **git clone https://github.com/mhartzel/ffcommander.git/**  
Go to source directory: **cd ffcommander.git**  
Build the program: **go build ffcommander.go**  


# Synopsis
ffcommander \[ options ] \[ file names ]  

# Video options
**-ac** Autocrop. Find crop values automatically by doing 10 second spot checks in 10 places for the duration of the file.  

**-crf** Use Constant Quality instead of 2-pass encoding. The default value for crf is 18, which produces the same quality as default 2-pass but a bigger file. CRF is much faster that 2-pass encoding.  

**-dn** Denoise. Use HQDN3D - filter to remove noise from the picture. This option is equal to Hanbrakes 'medium' noise reduction settings.

**-gr** Convert video to Grayscale. Use this option if the original source is black and white. This results more bitrate being available for b/w information and better picture quality.

**-it** Perform inverse telecine on 29.97 fps material to return it back to original 24 fps.

**-mbr** Override main videoprocessing automatic bitrate calculation and define bitrate manually.

**-nd** No Deinterlace. By default deinterlace is always used. This option disables it.

**-psd** Parallel SD. Create SD version in parallel to HD processing. This creates an additional version of the video downconverted to SD resolution. The SD file is stored in directory: sd

**-sbr** Override parallel sd videoprocessing automatic bitrate calculation and define bitrate manually. SD - video is stored in directory 'sd'

**-sf** Split out parts of the file. Give colon separated start and stop times for the parts of the file to use, for example: -sf 0,10:00,01:35:12.800,01:52:14 defines that 0 secs - 10 mins of the start of the file will be used and joined to the next part that starts at 01 hours 35 mins 12 seconds and 800 milliseconds and stops at 01 hours 52 mins 14 seconds. Don't use space - characters. A zero or word 'start' can be used to mark the absolute start of the file and word 'end' the end of the file. Both start and stop times must be defined.

**-ssd** Scale to SD. Scale video down to SD resolution. Calculates resolution automatically. Video is stored in directory 'sd'

**-tc** Burn timecode on top of the video. Timecode can be used to look for exact edit points for the file split feature

# Audio options
**-a** Audio language: -a fin or -a eng or -a ita  Find audio stream corresponding the language code. Only use option -an or -a not both.  

**-an** Audio stream number, -an 1. Only use option -an or -a not both.                 

**-ac3** Compress audio as ac3. Bitrate of 128k is used for each audio channel meaning 2 channels is compressed using 256k bitrate. 6 channels uses the ac3 max bitrate of 640k.  

**-aac** Compress audio as aac. Bitrate of 128k is used for each audio channel meaning 2 channels is compressed using 256k bitrate, 6 channels uses 768k bitrate.  

**-opus** Compress audio as opus. Opus support in mp4 container is experimental as of FFmpeg vesion 4.2.1. Bitrate of 128k is used for each audio channel meaning 2 channels is compressed using 256k bitrate, 6 channels uses 768k bitrate.  

**-flac** Compress audio in lossless Flac - format  

**-na** Disable audio processing. The resulting file will have no audio, only video.  

# Options affecting both audio and video
**-ls** Force encoding to use lossless 'utvideo' compression for video and 'flac' compression for audio. This also turns on -fe. This option only affects the main video if used with the -psd option.  

# Subtitle options
**-s** Burn subtitle with this language code on top of video. Example: -s fin or -s eng or -s ita  Only use option -sn or -s not both.  

**-sd** Subtitle `downscale`. When cropping video widthwise, scale down subtitle to fit on top of the cropped video instead of cropping the subtitle. This option results in smaller subtitle font. This option affects only subtitle burned on top of video.  

**-sgr** Subtitle Grayscale. Remove color from subtitle by converting it to grayscale. This option only works with subtitle burned on top of video. This option may also help if you experience jerky video every time subtitle picture changes.  

**-sn** Burn subtitle with this stream number on top of video. Example: -sn 1. Use subtitle number 1 from the source file. Only use option -sn or -s not both.  

**-so** Subtitle `offset`, -so 55 (move subtitle 55 pixels down), -so -55 (move subtitle 55 pixels up). This option affects only subtitle burned on top of video.  

**-sm** Mux subtitles with these language codes into the target file. Example: -sm eng, or -sm eng,fra,fin. This only works with dvd, dvb and bluray bitmap based subtitles. mp4 only supports DVD and DVB subtitles not Bluray. Bluray subtitles can be muxed into an mkv file using the -mkv option.  

**-smn** Mux subtitles with these stream numbers into the target file. Example: -smn 1 or -smn 3,1,7. This only works with dvd, dvb and bluray bitmap based subtitles. mp4 only supports DVD and DVB subtitles not Bluray. Bluray subtitles can be muxed into an mkv file using the -mkv option.  

**-palette** Hack dvd subtitle color palette. Option takes 1-16 comma separated hex numbers ranging from 0 to f. Zero = black, f = white, so only shades between black -> gray -> white can be defined. FFmpeg requires 16 hex numbers, so f's are automatically appended to the end of user given numbers. Each dvd uses color mapping differently so you need to try which numbers control the colors you want to change. Usually the first 4 numbers control the colors. Example: -palette f,0,f  This option affects only subtitle burned on top of video.  

**-sp** Subtile Split. Have you ever been annoyed when a subtitle is displayed on top of a actors face ? With this option you can automatically move subtitles further up and down at the edge of the screen. Distance from the screen edge will be picture height divided by 100 and rounded down to nearest integer. Minimum distance is 5 pixels and max 20 pixels. Subtitles will be automatically centered horizontally. You can also resize subtitles with the -sr option when usind Subtitle Split. The -sr option requires installing ImageMacick. The -sp option affects only subtitles burned on top of video.

**-sr** Subtitle Resize. Values less than 1 makes subtitles smaller, values bigger than 1 makes subtitle larger. This option can only be user with the -sp option. Example: make subtitle 25% smaller: -sr 0.75   make subtitle 50% smaller: -sr 0.50 make subtitle 75% larger: -sr 1.75. This option affects only subtitle burned on top of video.  

# Scan options                                                        
**-f** This is the same as using options -fs and -fe at the same time.  

**-fe** Fast encoding mode. Encode video using 1-pass encoding.  

**-fs** Fast seek mode. When using the -fs option with -st do not decode video before the point we are trying to locate, but instead try to jump directly to it. This search method might or might not be accurate depending on the file format.  

**-scan** Only scan input file and print video and audio stream info.  

**-st** Start time. Start video processing from this timecode. Example -st 30:00 starts processing from 30 minutes from the start of the file.  

**-et** End time. Stop video processing to this timecode. Example -et 01:30:00 stops processing at 1 hour 30 minutes. You can define a time range like this: -st 10:09 -et 01:22:49.500 This results in a video file that starts at 10 minutes 9 seconds and stops at 1 hour 22 minutes, 49 seconds and 500 milliseconds.  

**-d** Duration of video to process. Example -d 01:02 process 1 minutes and 2 seconds of the file. Use either -et or -d option not both.  

# Misc options
**-debug** Turn on debug mode and show info about internal variables and the FFmpeg commandlines used.  

**-mkv** Use matroska (mkv) as the output file wrapper format.  

**-print** Only print FFmpeg commands that would be used for processing, don't process any files.  

**-v ** Show the version of this program.  

**-version** Show the version of this program.  

**-td** Path to directory for temporary files, example_ -td PathToDir. This option directs temporary files created with 2-pass encoding and subtitle processing with the -sp switch to a separate directory. If the temp dir is a ram or a fast ssd disk then it speeds up processing with the -sp switch. Processing files with the -sp switch extracts every frame of the movie as a picture, so you need to have lots of space in the temp directory. For a FullHD movie you need to have 20 GB or more free storage. If you run multiple instances of this program simultaneously each instance processing one FullHD movie then you need 20 GB or more free storage for each movie that is processed at the same time. -sp switch extracts movie subtitle frames with FFmpeg and FFmpeg fails silently if it runs out of storage space. If this happens then some of the last subtitles won't be available when the video is compressed and this results the last available subtitle to be 'stuck' on top of video to the end of the movie.  

**-h** Display help text.
g
# Examples
Some examples of common usage.

# Exit Values
Sane exit values are still on my to do list.

# Why this program exists
I grew tired of using Handbrake because of it's limits and strange quirks. I've been using FFmpeg in my other projects (FreeLCS) and have become familiar with FFmpeg's immense power. There is not many things you can't do with it. But the commandline options become very complicated very fast when trying to do complex processing with it. FFcommander started as a shell script to automate creating these complex commandlines for FFmpeg. After getting tired of Bash's strange syntax I rewrote the program in Go and added features whenever I needed to do a new type of processing. Now a couple of years later I find the feature set quite big and unique and think that the program might be useful to other people as well. All this means that FFcommander still is my personal project and I might not accept any changes to it. Since it is released under GPL 3 you are welcome to make your own modifications or fork of it.

# Bugs
A list of known bugs and quirks. Sometimes, this is supplemented with (or replaced by) a link to the issue tracker for the project.

# Author and copyright
(C) 2018 Mikael Hartzell, Espoo, Finland.  
This program is distributed under the GNU General Public License version 3 (GPLv3)

# See Also
To find my other work visit: https://github.com/mhartzel There you can find FreeLCS that lets you automatically adjust audio loudness according to EBU R128 and my scripts to setup Vim as my C, C++, Go development environment (IDE).

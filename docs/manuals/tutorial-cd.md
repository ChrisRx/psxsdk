---
title: SCEE Tutorial: CD-ROM
weight: 11
---

Tutorial Version 1.0 - A quick guide to Net Yaroze programming

## CD-ROM Sample code

In the "chap3" directory you will find two examples of using the CD-ROM routines. The first example demonstrates loading files from the CD. The second example demonstrates playing back CD-DA tracks from your favourite audio CD.

## Loading Files from CD

### Setting up

To begin with, we will need a structure to hold information about the CD file. See listing 3.0 for the structure.

### Listing 3.0

    //------------------------------------------------------------------------------
    // S T R U C T U R E S
    //------------------------------------------------------------------------------

    typedef struct file_tag
     {

    	char file_name[32];
    	CdlFILE file_info;

     }file_data, *file_ptr;

    file_data cd_file[2];

The next step would be to store the CD file name in a variable so we can load the file. We simply use a string copy routine to copy the path of the CD file into our variable. Don’t forget to start the path with "\\" and end the CD file path with ";1". It is very important that you do this or our ReadFILE() routine won’t work properly. See listing 3.1 for setting up data files.

Listing 3.1

    // set-up for data files
    strcpy(cd_file[0].file_name,"\\DATA\\SOUND\\STD0.VH;1");
    strcpy(cd_file[1].file_name,"\\DATA\\SOUND\\STD0.VB;1");

Also don’t forget that we need a memory area to store these files in. Just define two memory locations in which our two files will be stored. See listing 3.2.

### Listing 3.2

    //--------------------------------------------------------------------------
    // D E F I N E S
    //--------------------------------------------------------------------------

    // snd file address
    #define VH_ADDR	 (0x80090000)
    #define VB_ADDR	 (0x800A0000)

Once we have set-up the file name, and address in which we want it to be stored, the file can then be read into the given address. Using the CdSearchFile() routine, it searches for the specified file and returns the location of the file along with the size. If the file is not found an error message will print out in your DOS debug window. After we have searched for the file and it is found, the file is then read using CdReadFile(). The file name along with the address which we want it stored, and the number of bytes to read are passed into CdReadFile(). There is also a error checker for checking any reading errors that might have occurred. This will also be displayed in your DOS debug window. Finally we wait for the termination of CdReadFile() by using the CdReadSync() routine in a while loop. [See Listing 5.3](chap5/1/main.html#Function:%20ReadFILE()) for the ReadFILE() routine.

### Listing 3.2

    //--------------------------------------------------------------------------
    // Function: ReadFILE()
    // Description: This routine searches for a single file, and stores that file
    //              at the address passed in
    // Parameters: *file: address of file information, *addr: address where file
    //             is stored
    // Returns: none
    // Notes: N/A
    //--------------------------------------------------------------------------

    void ReadFILE( file_data *file, u_long *addr )
     {

    	// get location and size from filename on CD-ROM
    	if( CdSearchFile(&file-file_info, file-file_name) == 0 )
    	  {
    		printf(" ERROR: %s not found \n", file-file_name);

    	  }

       	// read in file on CD-ROM
    	if( CdReadFile(file-file_name, addr, 0) == 0)
    	  {
    		printf(" ERROR: reading file \n");
    	  }

      	// wait for termination of CdRead
    	while( CdReadSync(1, 0)  0 );

     }// end ReadFILE

Below is the complete main() routine demonstrating loading files from a CD. With the simple press of button "X" the two files are loaded into the specified memory region of main ram. [See listing 3.4.](chap5/1/main.html)

### Listing 3.4

    //--------------------------------------------------------------------------
    // Function: main()
    // Description: CD Example 1 - Loading Files from CD
    // Parameters: none
    // Returns: int
    // Notes: N/A
    //--------------------------------------------------------------------------

    int main( void )
     {

        InitGame();

    	// main loop
        while( !DONE )
          {

    		 FntPrint(fnt_id[0], "~c900 Loading Files from CD Example \n~c990 Press X to load files ");
    	  	 FntPrint(fnt_id[1]," File name:%s", cd_file[0].file_info.name);
    		 FntPrint(fnt_id[2]," File size:%d bytes", cd_file[0].file_info.size);
    	  	 FntPrint(fnt_id[3]," File loc: min:%d, sec:%d, sector:%d, track:%d",
    	  	                      cd_file[0].file_info.pos.minute,
    	  	                      cd_file[0].file_info.pos.second,
    	  	                      cd_file[0].file_info.pos.sector,
    	  	                      cd_file[0].file_info.pos.track);

    	  	 FntPrint(fnt_id[4]," File name:%s", cd_file[1].file_info.name);
    		 FntPrint(fnt_id[5]," File size:%d bytes", cd_file[1].file_info.size);
    		 FntPrint(fnt_id[6]," File loc: min:%d, sec:%d, sector:%d, track:%d",
    	  	                      cd_file[1].file_info.pos.minute,
    	  	                      cd_file[1].file_info.pos.second,
    	  	                      cd_file[1].file_info.pos.sector,
    	  	                      cd_file[1].file_info.pos.track);

    		 if( PAD_PRESS(buffer1,PAD_CROSS) )
    		   {

    			 // set-up for data files
       	         	strcpy(cd_file[0].file_name,"\\DATA\\SOUND\\STD0.VH;1");
       	         	strcpy(cd_file[1].file_name,"\\DATA\\SOUND\\STD0.VB;1");

    			 ReadFILE(&cd_file[0],(u_long*)VH_ADDR);
    			 ReadFILE(&cd_file[1],(u_long*)VB_ADDR);

    		   }

    		 UpdateScreen();

          }// end while loop

        DeInitGame();   // de-init the game

        return(0);      // success

     }// end main

Playing CD-DA Tracks

### Setting up

In order to play CD-DA tracks we first need to set-up a few things. We are going to need 3 variables to hold our CD track list, the play mode and an offset into our track list. The track list is our "play list". Basically we will be playing tracks 1 through 9. We can easily modify our track list in whatever order we want. Let’s say we want to play in a random order and our music CD has "12" tracks. We could simply change our *tracks* array to {7,6,11,8,4,12,3,5,9,10,2,1,0}. Now our program will play the CD-DA tracks in that particular order. Notice the "0" that is at the end of the *tracks* array. The "0" is needed to notify the end of our track list. Our *mode* variable is set to "1" so our program will play our track list in the given order. Lastly our *offset* variable is set to "0" which points to the first track in our *tracks* array. We also have to set up our audio routines to be able to hear our CD-DA tracks. See listing 3.5 for more information.

### Listing 3.5

       	int tracks[11] = {1,2,3,4,5,6,7,8,9,0};
    	int mode=1;
    	int offset=0;
    	int count=0;

       	InitGame();

    	// set-up for CD-DA tracks
       	SsSetSerialAttr(SS_CD,SS_MIX,SS_SON);
       	SsSetSerialVol(SS_CD,127,127);

To play a CD-DA track we just use the routine CdPlay(). The CdPlay() routine needs a mode, a track list, and finally an offset into the track list. See Figure 3.6. The CdPlay() routine also provides four different modes. See Figure 3.7.

Figure 3.6

```text
┌─────────────────────────────────────────┬──────────────────────────────────────────┐
│CdPlay() Arguments                       │Description                               │
├─────────────────────────────────────────┼──────────────────────────────────────────┤
│Mode                                     │Mode (See Figure 3.7)                     │
├─────────────────────────────────────────┼──────────────────────────────────────────┤
│Tracks                                   │Array that specifies track to be played.  │
│                                         │Ends with 0.                              │
├─────────────────────────────────────────┼──────────────────────────────────────────┤
│Offset                                   │Index of tracks starting the performance  │
└─────────────────────────────────────────┴──────────────────────────────────────────┘
```

Figure 3.7

```text
┌─────────────────────────────────────────┬──────────────────────────────────────────┐
│Value                                    │Description                               │
├─────────────────────────────────────────┼──────────────────────────────────────────┤
│0                                        │Stops performance                         │
├─────────────────────────────────────────┼──────────────────────────────────────────┤
│1                                        │The tracks specified by tracks are played │
│                                         │in order until all tracks have been played│
├─────────────────────────────────────────┼──────────────────────────────────────────┤
│2                                        │Same as value 1 but all tracks are        │
│                                         │repeated in the same order                │
├─────────────────────────────────────────┼──────────────────────────────────────────┤
│3                                        │The index of the tracks array for the     │
│                                         │track currently being played is returned  │
└─────────────────────────────────────────┴──────────────────────────────────────────┘
```

To actually play, stop or skip a track in our *tracks* array we just call CdPlay() with the appropriate arguments. See Listing 3.8. When skipping tracks, we must take note on the track count in our *tracks* array. In this example there are only 9 CD-DA tracks that the user can actually play. A better way would be to let the user specify how many tracks their CD has since there is no routine to read the TOC ( table of contents) of the CD.

Listing 3.8

        // main loop
        while( !DONE )
          {

    		 FntPrint(fnt_id[0], "~c900 Playing CD-DA Tracks Example ");
    		 FntPrint(fnt_id[1], "~c990 Insert a Music disk and press any of the following: ");
    		 FntPrint(fnt_id[2], " Press O to stop \n Press X to play \n Press L1 to cycle tracks");
                     FntPrint(fnt_id[5]," Current Music Track: %d", offset+1);

    		 if( PAD_PRESS(buffer1,PAD_L1) )
    		   {
    		     // slow down the cycle
    		     if( (count+=7)  vsync_count )
    			   {

    		         count=0;

    		         if( offset <9 )
    			         offset++;
    			     else
    			         offset =0;
    			   }

    		     CdPlay(0,tracks,offset);  // cycle track

    		   }

    	  	 if( PAD_PRESS(buffer1,PAD_CIRCLE) )
    		   {
    		     CdPlay(0,tracks,offset);  // stop playing
    		   }

    		 if( PAD_PRESS(buffer1,PAD_CROSS) )
    		   {
    		     CdPlay(1,tracks,offset);  // start play
    		   }

    		 UpdateScreen();

          }// end while loop

        DeInitGame();   // de-init the game

        return(0);      // success

     }// end main

## Conclusion

In conclusion playing CD-DA tracks is fairly easy. The only things we must take note are the track list we set up and how many tracks the music CD actually has. When reading in data files from a CD we only need to worry where the files are going to be stored in memory. Once we have the memory location, the files can then be easily read in.

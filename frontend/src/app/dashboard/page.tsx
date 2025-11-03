"use client"

import { useState } from "react";
import { UserButton } from "@clerk/nextjs";
import { Cloud, Copy, Loader2 } from "lucide-react";
import { FileUpload } from "@/components/ui/file-upload";
import { toast } from "sonner";

type UploadedFile = {
  name: string;
  size: number;
  url: string;
};

function DashboardPage() {
  const [isUploading, setIsUploading] = useState(false);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [uploadedFiles, setUploadedFiles] = useState<UploadedFile[]>([]);

  // Handles copying the URL to the clipboard
  const handleCopyUrl = () => {
    if (!imageUrl) return;
    navigator.clipboard.writeText(imageUrl);
    toast.success("URL copied to clipboard!");
  };

  // Handles a file upload by sending it to a lambda endpoint.
  const handleFileUpload = async (files: File[]) => {
    const file = files[0];
    if (!file) return;

    setIsUploading(true);
    setImageUrl(null); // Reset previous URL

    // Read file as base64
    const toBase64 = (file: File): Promise<string> => {
      return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.readAsDataURL(file);
        reader.onload = () => {
          const result = reader.result as string;
          const base64Data = result.split(',')[1];
          resolve(base64Data);
        };
        reader.onerror = reject;
      });
    };

    try {
      const filedata = await toBase64(file);
      const payload = {
        filename: file.name,
        filedata,
        contenttype: file.type,
      };

      const response = await fetch(process.env.NEXT_PUBLIC_UPLOAD_ENDPOINT!, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({ message: "Failed to upload image" }));
        throw new Error(errorData.message || "Failed to upload image");
      }

      const { url } = await response.json();
      setImageUrl(url);

      // Add the uploaded file to the list
      setUploadedFiles((prevFiles) => [...prevFiles, { name: file.name, size: file.size, url }]);

      toast.success("Image uploaded successfully!");

    } catch (error: any) {
      console.error("Upload error:", error);
      toast.error(error.message || "An unknown error occurred.");
    } finally {
      setIsUploading(false);
    }
  };

  // A helper function to format bytes 
  const formatBytes = (bytes: number, decimals = 2) => {
    if (!+bytes) return '0 Bytes'
    const k = 1024
    const dm = decimals < 0 ? 0 : decimals
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`
  }

  // Calculate total storage used
  const totalStorageUsed = uploadedFiles.reduce((total, file) => total + file.size, 0);

  return (
    <div className="min-h-screen bg-background">
      {/* Header code */}
      <header className="border-b bg-white/50 backdrop-blur-sm sticky top-0 z-50">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center w-10 h-10 bg-blue-100 rounded-lg">
              <Cloud className="w-6 h-6 text-blue-600" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-gray-900">Nimbus</h1>
            </div>
          </div>
          <div className="flex items-center gap-4">
            <UserButton
              appearance={{
                elements: {
                  avatarBox: "w-10 h-10",
                  userButtonPopoverCard: "shadow-lg",
                },
              }}
              showName={false}
              userProfileMode="modal"
            />
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-8">
        <div className="max-w-4xl mx-auto">
          {/* Welcome Section */}
          <div className="text-center mb-8">
            <h2 className="text-3xl font-bold text-gray-900 mb-4">Welcome to Nimbus</h2>
            <p className="text-lg text-gray-600 max-w-2xl mx-auto">
              Convert your images to public URLs with blazing fast speed!
            </p>
          </div>


          {/* File Upload Section */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
            {/* <div className="p-6 border-b border-gray-100">
              <div className="flex items-center gap-3">
                <div className="flex items-center justify-center w-10 h-10 bg-blue-100 rounded-lg">
                  <Cloud className="w-5 h-5 text-blue-600" />
                </div>
                <div>
                  <h3 className="text-lg font-semibold text-gray-900">Upload File</h3>
                  <p className="text-sm text-gray-600">
                    {isUploading ? "Your file is being uploaded..." : "Drag and drop or click to browse"}
                  </p>
                </div>
              </div>
            </div> */}

            {/* Conditionally render FileUpload or Loading indicator */}
            <div className="p-6">
              {isUploading ? (
                <div className="flex flex-col items-center justify-center gap-4 py-10">
                  <Loader2 className="w-10 h-10 text-blue-600 animate-spin" />
                  <span className="text-gray-600">Please wait...</span>
                </div>
              ) : (
                <FileUpload onChange={handleFileUpload} />
              )}
            </div>
          </div>

          {/* URL Display Section */}
          {imageUrl && !isUploading && (
            <div className="mt-8">
              <h3 className="text-lg font-semibold text-gray-900 mb-4 text-center">Your URL is ready!</h3>
              <div className="relative flex items-center bg-white rounded-xl border border-gray-200 p-2 pr-12">
                <input
                  type="text"
                  value={imageUrl}
                  readOnly
                  className="w-full bg-transparent outline-none text-gray-700 px-2"
                />
                <button
                  onClick={handleCopyUrl}
                  className="absolute right-2 top-1/2 -translate-y-1/2 p-2 rounded-md bg-gray-100 hover:bg-gray-200 transition-colors"
                  aria-label="Copy URL"
                >
                  <Copy className="w-5 h-5 text-gray-600" />
                </button>
              </div>
            </div>
          )}

          {/* Stats & Recent Activity Sections */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mt-8">
            <div className="bg-white rounded-xl border border-gray-200 p-6 text-center hover:shadow-md transition-shadow">
              <div className="text-2xl font-bold text-blue-600 mb-2">{uploadedFiles.length}</div>
              <div className="text-sm text-gray-600">Files Uploaded</div>
            </div>
            <div className="bg-white rounded-xl border border-gray-200 p-6 text-center hover:shadow-md transition-shadow">
              <div className="text-2xl font-bold text-green-600 mb-2">{formatBytes(totalStorageUsed)}</div>
              <div className="text-sm text-gray-600">Storage Used</div>
            </div>
            <div className="bg-white rounded-xl border border-gray-200 p-6 text-center hover:shadow-md transition-shadow">
              <div className="text-2xl font-bold text-purple-600 mb-2"></div>
              <div className="text-sm text-gray-600">Shared Files</div>
            </div>
          </div>

          {/* Recent Activity Section */}
          <div className="mt-8 bg-white rounded-xl border border-gray-200 p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">Recent Activity</h3>

            {uploadedFiles.length === 0 ? (
              // Show this placeholder if no files are uploaded
              <div className="text-center py-8 text-gray-500">
                <Cloud className="w-12 h-12 mx-auto mb-3 text-gray-300" />
                <p>No recent activity</p>
                <p className="text-sm">Upload your first file to get started</p>
              </div>
            ) : (
              // Show the list of files if there are uploads
              <ul className="divide-y divide-gray-200">
                {uploadedFiles.map((file, index) => (
                  <li key={index} className="py-3 flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-gray-800">{file.name}</p>
                      <p className="text-xs text-gray-500">Size: {formatBytes(file.size)}</p>
                    </div>
                    <a href={file.url} target="_blank" rel="noopener noreferrer" className="text-sm text-blue-600 hover:underline">
                      View File
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </main>
    </div>
  );
};

export default DashboardPage;